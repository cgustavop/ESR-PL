package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
)

func main() {

	// Connect to the RP
	conn, err := net.Dial("tcp", "localhost:2345")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	args := os.Args

	//fmt.Fprintf(conn, args[1])

	// Write data to the connection
	_, err = conn.Write([]byte(args[1]))
	if err != nil {
		fmt.Println("Error writing:", err)
		return
	}
	fmt.Println("Sent:", args[1])

	println("Procuro o conteúdo " + args[1] + "...")

	/*
		// Listen for incoming connections.
		ln, err := net.Listen("tcp", "localhost:1234")
		if err != nil {
			log.Fatal(err)
		}
		defer ln.Close()

		// Accept connection from outside. This will block until a connection is made.
		println("TCP a espera...")
		connl, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		defer connl.Close()

		// Read from the connection. We'll stop when we're done or when we encounter an error.
		buffer := make([]byte, 1024)
		n, err := connl.Read(buffer)
		if err != nil {
			log.Fatal(err)
		}

		// Convert bytes to string and print it.
		message := string(buffer[:n])
		fmt.Println(message)
	*/

	// Read data from the connection
	buffer := make([]byte, 1024)
	bytesRead, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("Error reading:", err)
	}
	message := string(buffer[:bytesRead])
	fmt.Println("Received:", message)
	if message == args[1] {
		// show in player
		cmd := exec.Command("ffplay", "udp://localhost:1234")
		err := cmd.Run()
		print("cccccccccccc\n")
		println("Recieved from localhost:2345")
		if err != nil {
			log.Fatal(err)
		}

	}
}
