package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/fogleman/gg"
	"github.com/machinebox/sdk-go/objectbox"

	"gocv.io/x/gocv"
)

// Version is the version of the build
var Version = "dev"

type frame struct {
	number int
	buffer []byte
}

type endOfFile struct {
	frames int
}

func (e *endOfFile) Error() string {
	return "Video stream complete. frames: " + strconv.Itoa(e.frames)
}

func extractFrames(done <-chan struct{}, filename string) (<-chan frame, <-chan error) {
	framec := make(chan frame)
	errc := make(chan error, 1)

	go func() {
		defer close(framec)

		video, err := gocv.VideoCaptureFile(filename)
		if err != nil {
			errc <- err
			return
		}
		frameMat := gocv.NewMat()

		errc <- func() error {
			n := 1
			for {
				if !video.Read(&frameMat) {
					return &endOfFile{n}
				}
				buf, err := gocv.IMEncode(gocv.JPEGFileExt, frameMat)
				if err != nil {
					return err
				}
				select {
				case framec <- frame{n, buf}:
				case <-done:
					return errors.New("Frame extraction canceled")
				}
				n++
			}
		}()
	}()
	return framec, errc
}

type result struct {
	// the frame number
	frame int
	// The detected bounds
	detectors []objectbox.CheckDetectorResponse
	file      io.Reader
	err       error
}

func checker(done <-chan struct{}, frames <-chan frame, results chan<- result) {
	objectClient := objectbox.New("http://localhost:8083")
	info, err := objectClient.Info()
	if err != nil {
		log.Fatalf("could not get box info: %v", err)
	}
	log.Printf("Connected to box: %s %s %s %d", info.Build, info.Name, info.Status, info.Version)

	// process each frame from in channel
	for f := range frames {
		if f.number == 1 || f.number%10 == 0 {
			log.Printf("Processing frame %d\n", f.number)
		}
		// Set up a ReadWriter to hold the image sent to the model to write the file later.
		var bufferRead bytes.Buffer
		buffer := bytes.NewReader(f.buffer)
		tee := io.TeeReader(buffer, &bufferRead)
		resp, err := objectClient.Check(tee)
		detectors := make([]objectbox.CheckDetectorResponse, 0, len(resp.Detectors))
		// flatten detectors and identify found tags
		for _, t := range resp.Detectors {
			if len(t.Objects) > 0 {
				detectors = append(detectors, t)
			}
		}
		if len(detectors) == 0 {
			continue
		}
		select {
		case results <- result{f.number, detectors, &bufferRead, err}:
		case <-done:
			return
		}
	}
}

var filenameF string
var outputDirF string
var concurrencyF int
var versionF bool

func main() {
	flag.StringVar(&filenameF, "filename", "", "the file to process")
	flag.StringVar(&outputDirF, "outputDir", "rendered-frames", "the directory to place frames with detected objects")
	flag.IntVar(&concurrencyF, "concurrency", 10, "the number of concurrent frames to check")
	flag.BoolVar(&versionF, "v", false, "print the version")
	flag.Parse()

	if versionF {
		fmt.Println(Version)
		os.Exit(0)
	}

	if filenameF == "" {
		fmt.Println("-filename is required")
		os.Exit(1)
	}

	// done channel for cancellation
	done := make(chan struct{})
	defer close(done)

	// Generate the channel of frames from the video file
	log.Println("Start extracting frames")
	frames, errc := extractFrames(done, filenameF)

	// channel of frames with mice
	results := make(chan result)
	var wg sync.WaitGroup
	wg.Add(concurrencyF)

	// Process the frames by fanning out to `concurrency` workers.
	log.Println("Start processing frames")
	for i := 0; i < concurrencyF; i++ {
		go func() {
			checker(done, frames, results)
			wg.Done()
		}()
	}

	// when each all workers are done, close the results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			log.Printf("Frame result with an error: %v\n", r.err)
			continue
		}
		log.Printf("Mouse detected! frame: %d, detectors: %v\n", r.frame, r.detectors)

		image, err := jpeg.Decode(r.file)
		if err != nil {
			log.Printf("Unable to decode image: %v", err)
			continue
		}

		imgCtx := gg.NewContextForImage(image)
		green := color.RGBA{0, 100, 0, 255}
		imgCtx.SetColor(color.Transparent)
		imgCtx.SetStrokeStyle(gg.NewSolidPattern(green))
		imgCtx.SetLineWidth(2)

		for _, d := range r.detectors {
			left := float64(d.Objects[0].Rect.Left)
			top := float64(d.Objects[0].Rect.Top)
			width := float64(d.Objects[0].Rect.Width)
			height := float64(d.Objects[0].Rect.Height)
			imgCtx.DrawRectangle(left, top, width, height)
			imgCtx.Stroke()
		}

		cleanedFilename := strings.ReplaceAll(filenameF, "/", "-")
		frameFile := path.Join(outputDirF, Version+"-"+cleanedFilename+"-"+strconv.Itoa(r.frame)+".jpg")

		err = gg.SaveJPG(frameFile, imgCtx.Image(), 100)
		if err != nil {
			log.Printf("Unable to create image: %v\n", err)
			continue
		}
	}
	if err := <-errc; err != nil {
		switch err.(type) {
		case *endOfFile:
			log.Printf("Finished processing video: %v\n", err)
		default:
			log.Fatalf("Error detected: %v\n", err)
		}
	}
}
