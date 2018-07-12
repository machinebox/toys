package main

import (
	"bufio"
	"context"
	"encoding/base64"
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
		teachratio = flag.Float64("teachratio", 0.8, "ratio of images to teach vs use for validation")
		passes     = flag.Int("passes", 1, "number of times to teach the examples")
	)
	flag.Parse()
	cb := classificationbox.New(*cbAddr)
	info, err := cb.Info()
	if err != nil {
		return errors.Wrap(err, "cannot find Classificationbox")
	}
	if info.Name != "classificationbox" {
		return errors.New("Classificationbox not running on " + *cbAddr)
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
	images := newImageExamples(classes)
	shuffle(images, randomSource)
	teachImagesCount := int(float64(len(images)) * *teachratio)
	if !readYorN(fmt.Sprintf("Teach and validate Classificationbox with %d (%g%%) random images? (y/n): ", teachImagesCount, teachratioperc)) {
		return errors.New("aborted")
	}
	teachImages, validateImages := split(randomSource, teachImagesCount, images)
	for i := 0; i < *passes; i++ {
		fmt.Printf("  pass %d of %d...\n", i+1, *passes)
		if err := teach(ctx, cb, model.ID, teachImages); err != nil {
			return errors.Wrap(err, "teaching")
		}
	}
	fmt.Println("waiting for teaching to complete...")
	fmt.Println()
	time.Sleep(5 * time.Second)
	if err := validate(ctx, cb, model.ID, validateImages); err != nil {
		return errors.Wrap(err, "validating")
	}
	return nil
}

func teach(ctx context.Context, cb *classificationbox.Client, modelID string, images []imageExample) error {
	fmt.Print("teaching: ")
	bar := pb.StartNew(len(images))
	for _, image := range images {
		if err := teachImage(ctx, cb, modelID, image); err != nil {
			fmt.Printf("Error teaching: %s", err)
			fmt.Println("Pressing onward...")
		}
		bar.Increment()
	}
	bar.FinishPrint("Teaching complete")
	return nil
}

func teachImage(ctx context.Context, cb *classificationbox.Client, modelID string, image imageExample) error {
	base64, err := base64Image(image.path)
	if err != nil {
		return err
	}
	example := classificationbox.Example{
		Class: image.class,
		Inputs: []classificationbox.Feature{
			classificationbox.FeatureImageBase64("image", base64),
		},
	}
	if err := cb.Teach(ctx, modelID, example); err != nil {
		return err
	}
	return nil
}

func validate(ctx context.Context, cb *classificationbox.Client, modelID string, images []imageExample) error {
	fmt.Print("validating...")
	bar := pb.StartNew(len(images))
	var correct, incorrect, errors int
	for _, image := range images {
		predictedClass, err := predictImage(ctx, cb, modelID, image)
		if err != nil {
			errors++
			//fmt.Print("!")
			continue
		}
		if predictedClass == image.class {
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
	acc := float64(correct) / float64(len(images))
	fmt.Printf("Accuracy:   %g%%\n", acc*100)
	fmt.Println()
	return nil
}

func predictImage(ctx context.Context, cb *classificationbox.Client, modelID string, image imageExample) (string, error) {
	base64, err := base64Image(image.path)
	if err != nil {
		return "", err
	}
	req := classificationbox.PredictRequest{
		Inputs: []classificationbox.Feature{
			classificationbox.FeatureImageBase64("image", base64),
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
		imagefiles, err := ioutil.ReadDir(filepath.Join(src, dir.Name()))
		if err != nil {
			return nil, errors.Wrap(err, dir.Name())
		}
		for _, imageFile := range imagefiles {
			if imageFile.IsDir() || skip(imageFile.Name()) {
				continue // skip dirs
			}
			classes[dir.Name()] = append(classes[dir.Name()], filepath.Join(src, dir.Name(), imageFile.Name()))
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
	var totalImages int
	for _, images := range classes {
		totalImages += len(images)
	}
	// check to ensure the classes are more or less balanced
	// i.e. number of images should be within 10% of average
	averageImages := totalImages / len(classes)
	for class, images := range classes {
		fmt.Printf("%s:\t%d image(s) ", class, len(images))
		ratio := float64(averageImages) / float64(len(images))
		if ratio <= 0.95 || ratio >= 1.05 {
			fmt.Print("\tWARNING: Classes should be balanced")
		} else if len(images) < 10 {
			fmt.Print("\tWARNING: Low number of images")
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

// imageExample is an image example.
type imageExample struct {
	path  string
	class string
}

func newImageExamples(classes map[string][]string) []imageExample {
	var imageExamples []imageExample
	for class, images := range classes {
		for _, imagePath := range images {
			imageExamples = append(imageExamples, imageExample{
				class: class,
				path:  imagePath,
			})
		}
	}
	return imageExamples
}

func split(randomSource rand.Source, teachCount int, imageExamples []imageExample) (teach []imageExample, validate []imageExample) {
	random := rand.New(randomSource)
	var teachImages []imageExample
	teachImages = append(teachImages, imageExamples...)
	var validateImages []imageExample
	for len(teachImages) > teachCount {
		i := random.Intn(len(teachImages))
		validateImages = append(validateImages, teachImages[i])
		teachImages = append(teachImages[:i], teachImages[i+1:]...)
	}
	return teachImages, validateImages
}

func shuffle(images []imageExample, randomSource rand.Source) {
	random := rand.New(randomSource)
	for i := len(images) - 1; i > 0; i-- {
		j := random.Intn(i + 1)
		images[i], images[j] = images[j], images[i]
	}
}

func base64Image(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}
