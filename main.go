package main

import (
	"bytes"
	"errors"
	"log"
	"sync"

	"github.com/machinebox/sdk-go/objectbox"
	"gocv.io/x/gocv"
)

type frame struct {
	number int
	buffer []byte
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
					return errors.New("Unable to read frame")
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
		resp, err := objectClient.Check(bytes.NewReader(f.buffer))
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
		case results <- result{f.number, detectors, err}:
		case <-done:
			return
		}
	}
}

func main() {

	filename := "record/20200605/03/53.mp4"

	// done channel for cancellation
	done := make(chan struct{})
	defer close(done)

	// Generate the channel of frames from the video file
	log.Println("Start extracting frames")
	frames, errc := extractFrames(done, filename)

	// channel of frames with mice
	results := make(chan result)
	var wg sync.WaitGroup
	const concurrency = 10 // TODO:grocky convert to flag
	wg.Add(concurrency)

	// Process the frames by fanning out to `concurrency` workers.
	log.Println("Start processing frames")
	for i := 0; i < concurrency; i++ {
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
		log.Printf("Mouse detected! frame: %d, detectors: %v", r.frame, r.detectors)
	}
	if err := <-errc; err != nil {
		log.Fatalf("Error detected: %v", err)
	}
}
