package main

import (
	"log"
	"os/exec"
)

func main() {
	cmd := exec.Command("ffplay", "udp://localhost:1234")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}
