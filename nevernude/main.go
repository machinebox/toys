package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/machinebox/sdk-go/videobox"
	"github.com/pkg/errors"
)

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	fmt.Println(`
nevernude by Machine Box
Powered by Videobox + Nudebox

https://machinebox.io/
@machineboxio
`)
	var (
		threshold    = flag.Float64("threshold", 0.4, "nudebox threshold (lower is more strict)")
		videoboxAddr = flag.String("videobox", "http://localhost:8080", "Videobox address")
		outFile      = flag.String("out", "", "output file (default will save file next to original)")
		skipFrames   = flag.Int("skipframes", -1, "number of frames to skip between extractions (see Videobox docs)")
		skipSeconds  = flag.Int("skipseconds", -1, "number of seconds to skip between extractions (see Videobox docs)")
	)
	flag.Parse()
	if *threshold < 0 || *threshold > 1 {
		return errors.New("threshold must be between 0 and 1")
	}
	args := flag.Args()
	inFile := args[0]
	ext := filepath.Ext(inFile)
	localtmp := fmt.Sprintf(".nevernude-%d", time.Now().Unix())
	tmpdir := filepath.Join(localtmp, filepath.Base(inFile))
	if err := os.MkdirAll(tmpdir, 0777); err != nil {
		return errors.Wrap(err, "make temp directory")
	}
	defer func() {
		os.RemoveAll(localtmp)
	}()
	f, err := os.Open(inFile)
	if err != nil {
		return errors.New("open video")
	}
	defer f.Close()
	fmt.Println("posting file to Videobox...")
	vb := videobox.New(*videoboxAddr)
	opts := videobox.NewCheckOptions()
	opts.NudeboxThreshold(*threshold)
	if *skipFrames > -1 {
		opts.SkipFrames(*skipFrames)
	}
	if *skipSeconds > -1 {
		opts.SkipSeconds(*skipSeconds)
	}
	video, err := vb.Check(f, opts)
	if err != nil {
		return errors.Wrap(err, "videobox check")
	}
	fmt.Println("waiting for Videobox...")
	results, video, err := waitForVideoboxResults(vb, video.ID)
	if err != nil {
		return errors.Wrap(err, "waiting for results")
	}
	fmt.Println("processing...")
	var keepranges []rangeMS
	offsetMS := 500 // buffer around the nudity
	s := 0
	for _, nudity := range results.Nudebox.Nudity {
		for _, instance := range nudity.Instances {
			r := rangeMS{
				Start: s - offsetMS,
				End:   instance.StartMS + offsetMS,
			}
			s = instance.EndMS
			keepranges = append(keepranges, r)
		}
	}
	keepranges = append(keepranges, rangeMS{
		Start: s,
		End:   video.MillisecondsComplete,
	})
	ffmpegargs := []string{
		"-y", "-i", inFile,
	}
	listFileName := filepath.Join(tmpdir, "segments.txt")
	lf, err := os.Create(listFileName)
	if err != nil {
		return errors.Wrap(err, "create list file")
	}
	defer lf.Close()
	for i, r := range keepranges {
		start := strconv.Itoa(r.Start / 1000)
		duration := strconv.Itoa((r.End - r.Start) / 1000)
		segmentFile := fmt.Sprintf("%04d_%s-%s%s", i, start, start+duration, ext)
		segment := filepath.Join(tmpdir, segmentFile)
		if _, err := io.WriteString(lf, "file '"+segmentFile+"'\n"); err != nil {
			return errors.Wrap(err, "writing to list file")
		}
		ffmpegargs = append(ffmpegargs, []string{
			"-ss", start,
			"-t", duration,
			segment,
		}...)
	}
	fmt.Printf("breaking videos into %d segment(s)... (this can take a while)\n", len(keepranges))
	out, err := exec.Command("ffmpeg", ffmpegargs...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "ffpmeg: "+string(out))
	}
	output := *outFile
	if output == "" {
		output = inFile[:len(inFile)-len(ext)] + "-nevernude" + ext
	}
	fmt.Println("stitching segments into", output+"...")
	ffmpegargs = []string{
		"-y", "-f", "concat", "-safe", "0", "-i", listFileName, "-c", "copy", output,
	}
	out, err = exec.Command("ffmpeg", ffmpegargs...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "ffpmeg: "+string(out))
	}
	fmt.Println("done.")
	return nil
}

type rangeMS struct {
	Start, End int
}

func waitForVideoboxResults(vb *videobox.Client, id string) (*videobox.VideoAnalysis, *videobox.Video, error) {
	var video *videobox.Video
	err := func() error {
		defer fmt.Println()
		for {
			time.Sleep(2 * time.Second)
			var err error
			video, err = vb.Status(id)
			if err != nil {
				return err
			}
			switch video.Status {
			case videobox.StatusComplete:
				return nil
			case videobox.StatusFailed:
				return errors.New("videobox: " + video.Error)
			}
			perc := float64(100) * (float64(video.FramesComplete) / float64(video.FramesCount))
			if perc < 0 {
				perc = 0
			}
			if perc > 100 {
				perc = 100
			}
			fmt.Printf("\r%d%% complete...", int(perc))
		}
	}()
	if err != nil {
		return nil, video, err
	}
	results, err := vb.Results(id)
	if err != nil {
		return nil, video, errors.Wrap(err, "get results")
	}
	if err := vb.Delete(id); err != nil {
		log.Println("videobox: failed to delete results (continuing regardless):", err)
	}
	return results, video, nil
}
