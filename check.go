package main

import (
	"bytes"
	"io"
	"log"

	"github.com/machinebox/sdk-go/objectbox"
)

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
