package main

import (
	"flag"
	"fmt"
	"image/color"
	"image/jpeg"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/fogleman/gg"
)

// Version is the version of the build
var Version = "dev"

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
	log.Println("Start extracting frames from the video")
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

	processResults(results)

	if err := <-errc; err != nil {
		switch err.(type) {
		case *endOfFile:
			log.Printf("Finished processing video: %v\n", err)
		default:
			log.Fatalf("Error detected: %v\n", err)
		}
	}
}

func processResults(results <-chan result) {
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
		green := color.RGBA{50, 205, 50, 255}
		imgCtx.SetColor(color.Transparent)
		imgCtx.SetStrokeStyle(gg.NewSolidPattern(green))
		imgCtx.SetLineWidth(1)

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
}
