package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/machinebox/sdk-go/boxutil"
	"github.com/machinebox/sdk-go/classificationbox"
	"github.com/pkg/errors"
	pb "gopkg.in/cheggaaa/pb.v1"
)

func main() {
	ctx := context.Background()
	// trap Ctrl+C and call cancel on the context
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()
	if err := run(ctx); err != nil {
		log.Fatalln(err)
	}
}

func run(ctx context.Context) error {
	var (
		cbAddr     = flag.String("cb", "http://localhost:8080", "Classificationbox address")
		src        = flag.String("src", ".", "source of dataset")
		teachratio = flag.Float64("teachratio", 0.8, "ratio of items to teach vs use for validation")
		passes     = flag.Int("passes", 1, "number of times to teach the examples")
	)
	flag.Parse()
	cb := classificationbox.New(*cbAddr)
	info, err := cb.Info()
	if err != nil {
		return errors.Wrap(err, "cannot find Classificationbox")
	}
	if info.Name != "classificationbox" {
		return errors.New("Classificationbox not running on " + *cbAddr + ". Go to https://machinebox.io/account to get started.")
	}
	if err := boxutil.WaitForReady(ctx, cb); err != nil {
		return err
	}
	absSrc, abserr := filepath.Abs(*src)
	if abserr != nil {
		absSrc = *src
	}
	absSrcLocation := filepath.Join(absSrc, "*")
	classes, err := collectTrainingData(ctx, *src)
	if err != nil {
		return errors.Wrap(err, "classes data")
	}
	if err := validateClasses(classes); err != nil {
		return errors.Wrap(err, absSrcLocation)
	}
	var classNames []string
	for class := range classes {
		classNames = append(classNames, class)
	}
	if !readYorN(fmt.Sprintf("Create new model with %d classes? (y/n): ", len(classNames))) {
		return errors.New("aborted")
	}
	model := classificationbox.Model{
		Classes: classNames,
	}
	model, err = cb.CreateModel(ctx, model)
	if err != nil {
		return errors.Wrap(err, "create model")
	}
	fmt.Printf("new model created: %s\n", model.ID)
	teachratioperc := *teachratio * 100.0
	randomSource := rand.NewSource(time.Now().UnixNano())
	items := newitemExamples(classes)
	shuffle(items, randomSource)
	teachitemsCount := int(float64(len(items)) * *teachratio)
	if !readYorN(fmt.Sprintf("Teach and validate Classificationbox with %d (%g%%) random items? (y/n): ", teachitemsCount, teachratioperc)) {
		return errors.New("aborted")
	}
	teachitems, validateitems := split(randomSource, teachitemsCount, items)
	for i := 0; i < *passes; i++ {
		fmt.Printf("  pass %d of %d...\n", i+1, *passes)
		if err := teach(ctx, cb, model.ID, teachitems); err != nil {
			return errors.Wrap(err, "teaching")
		}
	}
	fmt.Println("waiting for teaching to complete...")
	fmt.Println()
	time.Sleep(5 * time.Second)
	if err := validate(ctx, cb, model.ID, validateitems); err != nil {
		return errors.Wrap(err, "validating")
	}
	return nil
}

func teach(ctx context.Context, cb *classificationbox.Client, modelID string, items []itemExample) error {
	fmt.Print("teaching: ")
	bar := pb.StartNew(len(items))
	for _, item := range items {
		if err := teachitem(ctx, cb, modelID, item); err != nil {
			fmt.Printf("Error teaching: %s", err)
			fmt.Println("Pressing onward...")
		}
		bar.Increment()
	}
	bar.FinishPrint("Teaching complete")
	return nil
}

func teachitem(ctx context.Context, cb *classificationbox.Client, modelID string, item itemExample) error {
	content, err := loadItem(item.path)
	if err != nil {
		return err
	}
	example := classificationbox.Example{
		Class: item.class,
		Inputs: []classificationbox.Feature{
			classificationbox.FeatureText("item", content),
		},
	}
	if err := cb.Teach(ctx, modelID, example); err != nil {
		return err
	}
	return nil
}

func loadItem(src string) (string, error) {
	b, err := ioutil.ReadFile(src)
	if err != nil {
		return "", err
	}
	return string(b), err
}

func validate(ctx context.Context, cb *classificationbox.Client, modelID string, items []itemExample) error {
	fmt.Print("validating...")
	bar := pb.StartNew(len(items))
	var correct, incorrect, errors int
	for _, item := range items {
		predictedClass, err := predictitem(ctx, cb, modelID, item)
		if err != nil {
			errors++
			//fmt.Print("!")
			continue
		}
		if predictedClass == item.class {
			correct++
			//fmt.Print("✓")
		} else {
			incorrect++
			//fmt.Print("𐄂")
		}
		bar.Increment()
	}
	bar.FinishPrint("Validation complete")
	fmt.Println()
	fmt.Printf("Correct:    %d\n", correct)
	fmt.Printf("Incorrect:  %d\n", incorrect)
	fmt.Printf("Errors:     %d\n", errors)
	acc := float64(correct) / float64(len(items))
	fmt.Printf("Accuracy:   %g%%\n", acc*100)
	fmt.Println()
	return nil
}

func predictitem(ctx context.Context, cb *classificationbox.Client, modelID string, item itemExample) (string, error) {
	content, err := loadItem(item.path)
	if err != nil {
		return "", err
	}
	req := classificationbox.PredictRequest{
		Inputs: []classificationbox.Feature{
			classificationbox.FeatureText("item", content),
		},
	}
	resp, err := cb.Predict(ctx, modelID, req)
	if err != nil {
		return "", errors.Wrap(err, "predict")
	}
	return resp.Classes[0].ID, nil
}

func collectTrainingData(ctx context.Context, src string) (map[string][]string, error) {
	classdirs, err := ioutil.ReadDir(src)
	if err != nil {
		return nil, err
	}
	classes := make(map[string][]string)
	for _, dir := range classdirs {
		if !dir.IsDir() || skip(dir.Name()) {
			continue // skip files
		}
		files, err := ioutil.ReadDir(filepath.Join(src, dir.Name()))
		if err != nil {
			return nil, errors.Wrap(err, dir.Name())
		}
		for _, file := range files {
			if file.IsDir() || skip(file.Name()) {
				continue // skip dirs
			}
			classes[dir.Name()] = append(classes[dir.Name()], filepath.Join(src, dir.Name(), file.Name()))
		}
	}
	return classes, nil
}

func validateClasses(classes map[string][]string) error {
	if len(classes) < 2 {
		return errors.New("you need at least two classes")
	}
	fmt.Println()
	fmt.Println("Classes")
	fmt.Println("-------")
	var totalitems int
	for _, items := range classes {
		totalitems += len(items)
	}
	// check to ensure the classes are more or less balanced
	// i.e. number of items should be within 10% of average
	averageitems := totalitems / len(classes)
	for class, items := range classes {
		fmt.Printf("%s:\t%d item(s) ", class, len(items))
		ratio := float64(averageitems) / float64(len(items))
		if ratio <= 0.95 || ratio >= 1.05 {
			fmt.Print("\tWARNING: Classes should be balanced")
		} else if len(items) < 10 {
			fmt.Print("\tWARNING: Low number of items")
		}
		fmt.Println()
	}
	fmt.Println()
	return nil
}

func skip(path string) bool {
	if strings.HasPrefix(filepath.Base(path), ".") {
		return true
	}
	return false
}

func readYorN(prompt string) bool {
	fmt.Print(prompt)
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		switch strings.ToLower(s.Text()) {
		case "y":
			return true
		case "n":
			return false
		default:
			fmt.Print(prompt)
		}
	}
	return false
}

// itemExample is an item example.
type itemExample struct {
	path  string
	class string
}

func newitemExamples(classes map[string][]string) []itemExample {
	var itemExamples []itemExample
	for class, items := range classes {
		for _, itemPath := range items {
			itemExamples = append(itemExamples, itemExample{
				class: class,
				path:  itemPath,
			})
		}
	}
	return itemExamples
}

func split(randomSource rand.Source, teachCount int, itemExamples []itemExample) (teach []itemExample, validate []itemExample) {
	random := rand.New(randomSource)
	var teachitems []itemExample
	teachitems = append(teachitems, itemExamples...)
	var validateitems []itemExample
	for len(teachitems) > teachCount {
		i := random.Intn(len(teachitems))
		validateitems = append(validateitems, teachitems[i])
		teachitems = append(teachitems[:i], teachitems[i+1:]...)
	}
	return teachitems, validateitems
}

func shuffle(items []itemExample, randomSource rand.Source) {
	random := rand.New(randomSource)
	for i := len(items) - 1; i > 0; i-- {
		j := random.Intn(i + 1)
		items[i], items[j] = items[j], items[i]
	}
}
