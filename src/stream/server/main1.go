package main

import (
	"fmt"
	"log"
	"net"
	"os/exec"
)

func main() {
	// Listen for incoming connections.
	ln, err := net.Listen("tcp", "localhost:3456")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	// Accept connection from outside. This will block until a connection is made.
	conn, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Read from the connection. We'll stop when we're done or when we encounter an error.
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Fatal(err)
	}

	// Convert bytes to string and print it.
	message := string(buffer[:n])
	fmt.Println(message)

	println("Vou transmitir o conteúdo: " + message + "!")

	// Connect to the server
	conns, err := net.Dial("tcp", "localhost:2345")
	if err != nil {
		log.Fatal(err)
	}
	defer conns.Close()

	fmt.Fprintf(conns, message)

	//time.Sleep(2 * time.Second)

	cmd := exec.Command("ffmpeg", "-stream_loop", "-1", "-i", message, "-pix_fmt", "yuvj422p", "-s", "640x360", "-r", "25", "-c:v", "mjpeg", "-f", "mjpeg", "udp://localhost:2345")
	err = cmd.Run()
	println("Sent to localhost:2345")
	if err != nil {
		log.Fatal(err)
	}
}
