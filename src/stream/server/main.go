package main

import (
	"log"
	"os/exec"
)

func main() {
	cmd := exec.Command("ffmpeg", "-stream_loop", "-1", "-i", "safira.mjpeg.avi", "-pix_fmt", "yuvj422p", "-s", "640x360", "-r", "25", "-c:v", "mjpeg", "-f", "mjpeg", "udp://localhost:2345")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}
