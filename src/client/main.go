package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	filePath := os.Args[1]

	// abrir a socket para receber pedidos
	adr := "localhost:8080"
	udpAddr, _ := net.ResolveUDPAddr("udp", adr)
	listener, erro := net.DialUDP("udp", nil, udpAddr)
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer listener.Close()

	fmt.Printf("Client is connected at %s\n", adr)

	// manda cenas ao server
	mandaCenas(listener, filePath)
	fmt.Printf("Sent request for %s\n", filePath)

	// recebe cenas do server
	recebeCenas(listener, filePath)

	fmt.Printf("Closing connection...\n")
}

func mandaCenas(conn *net.UDPConn, text string) {
	_, err := conn.Write([]byte(text))
	if err != nil {
		fmt.Println("Error sending message:", err)
		return
	}
}

func recebeCenas(conn *net.UDPConn, filePath string) {

	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	// Set buffer size
	bufferSize := 1024
	buffer := make([]byte, bufferSize)

	// Loop to receive and write packets to the file
	for {
		// Receive a packet from the sender
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error receiving data:", err)
			return
		}

		var buf [1024]byte
		data := string(buf[:n])
		if data == "EOF" {
			fmt.Println("File received successfully.")
			_, err = conn.Write([]byte("EOF"))
			if err != nil {
				fmt.Println("Error sending message:", err)
			}
			return
		} else {

			// Write the received data to the file
			_, err = file.Write(buffer[:n])
			if err != nil {
				fmt.Println("Error writing to file:", err)
				return
			}

			_, err = conn.Write([]byte("ACK"))
			if err != nil {
				fmt.Println("Error sending message:", err)
				return
			}
		}
	}
}
