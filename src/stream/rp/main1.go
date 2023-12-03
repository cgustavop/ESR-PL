package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	// Listen for incoming connections.
	ln, err := net.Listen("tcp", "localhost:2345")
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

	// Connect to the server
	conns, err := net.Dial("tcp", "localhost:3456")
	if err != nil {
		log.Fatal(err)
	}
	defer conns.Close()

	fmt.Fprintf(conns, message)

	println("Quero o conteúdo " + message + "...")

	// Accept connection from outside. This will block until a connection is made.
	connl, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}
	defer connl.Close()

	// Read from the connection. We'll stop when we're done or when we encounter an error.
	buffer = make([]byte, 1024)
	n, err = connl.Read(buffer)
	if err != nil {
		log.Fatal(err)
	}

	// Convert bytes to string and print it.
	message2 := string(buffer[:n])
	fmt.Println(message2)

	if message2 == message {
		/*
			// Connect to the server
			println("AAAAAAAAAAAAAAAA")
			connst, err := net.Dial("tcp", "localhost:1234")
			if err != nil {
				log.Fatal(err)
			}
			println("BBBBBBBBBBBBBBBB")
			defer connst.Close()

			fmt.Fprintf(connst, message2)
			time.Sleep(2 * time.Second)
		*/

		// Write data to the connection
		_, err = conn.Write([]byte(message2))
		if err != nil {
			fmt.Println("Error writing:", err)
			return
		}
		fmt.Println("Sent:", message2)

		// Resolve UDP address Server
		addr, err := net.ResolveUDPAddr("udp", "localhost:2345")
		if err != nil {
			log.Fatal(err)
		}

		// Listen for UDP connections server
		connu, err := net.ListenUDP("udp", addr)
		if err != nil {
			log.Fatal(err)
		}
		defer connu.Close()

		println("AAAAAAAAAAAAAA")

		// Resolve UDP address for redirection cliente
		redirAddr, err := net.ResolveUDPAddr("udp", "localhost:1234")
		if err != nil {
			log.Fatal(err)
		}

		// Dial UDP for redirection client
		redirConn, err := net.DialUDP("udp", nil, redirAddr)
		if err != nil {
			log.Fatal(err)
		}
		defer redirConn.Close()

		// Buffer to store incoming data
		buf := make([]byte, 2048)

		// Continuously read data from the connection and redirect it
		for {
			n, _, err := connu.ReadFromUDP(buf)
			if err != nil {
				log.Fatal(err)
			}
			println("Recieved from localhost:2345")

			// Redirect data
			_, err = redirConn.Write(buf[:n])
			if err != nil {
				log.Fatal(err)
			}
			println("Sent to localhost:1234")
		}
	}
}
