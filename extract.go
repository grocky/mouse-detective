package main

import (
	"errors"
	"strconv"

	"gocv.io/x/gocv"
)

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
