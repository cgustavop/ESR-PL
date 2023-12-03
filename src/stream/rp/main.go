package main

import (
	"log"
	"net"
)

func main() {
	// Resolve UDP address
	addr, err := net.ResolveUDPAddr("udp", "localhost:2345")
	if err != nil {
		log.Fatal(err)
	}

	// Listen for UDP connections
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Resolve UDP address for redirection
	redirAddr, err := net.ResolveUDPAddr("udp", "localhost:1234")
	if err != nil {
		log.Fatal(err)
	}

	// Dial UDP for redirection
	redirConn, err := net.DialUDP("udp", nil, redirAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer redirConn.Close()

	// Buffer to store incoming data
	buf := make([]byte, 2048)

	// Continuously read data from the connection and redirect it
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Fatal(err)
		}

		// Redirect data
		_, err = redirConn.Write(buf[:n])
		if err != nil {
			log.Fatal(err)
		}
	}
}
