package main

import (
	"bytes"
	"log"

	"github.com/machinebox/sdk-go/objectbox"
	"gocv.io/x/gocv"
)

func main() {
	objectClient := objectbox.New("http://localhost:8083")
	info, err := objectClient.Info()
	if err != nil {
		log.Fatalf("could not get box info: %v", err)
	}
	log.Printf("Connected to box: %s %s %s %d", info.Build, info.Name, info.Status, info.Version)

	filename := "record/20200605/03/53.mp4"
	video, _ := gocv.VideoCaptureFile(filename)
	img := gocv.NewMat()

	for {
		video.Read(&img)
		image, err := gocv.IMEncode(gocv.JPEGFileExt, img)
		if err != nil {
			log.Fatalf("Unable to encode frame: %v", err)
		}
		resp, err := objectClient.Check(bytes.NewReader(image))
		if err != nil {
			log.Println("Check failed for frame")
			continue
		}
		log.Printf("%v", resp.Detectors)
	}

}
