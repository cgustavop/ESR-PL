package main

import (
	"net"
	"os"
)

const BUFSIZE = 1024 * 8

// Adapted from https://github.com/rb-de0/go-mp4-stream
func streamRequest(conn *net.UDPConn, client *net.UDPAddr, filePath string) {
	file, err := os.Open(filePath)

	if err != nil {
		// ERROOOO
		return
	}

	defer file.Close()

	fi, err := file.Stat()

	if err != nil {
		// ERROOOO
		return
	}

	//fileSize := int(fi.Size())

	buffer := make([]byte, BUFSIZE)

	for {
		n, err := file.Read(buffer)

		if n == 0 {
			break
		}

		if err != nil {
			break
		}

		data := buffer[:n]
		conn.WriteToUDP(data, client)
	}
}
