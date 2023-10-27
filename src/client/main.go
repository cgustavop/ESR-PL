package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	// abrir a socket para receber pedidos
	adr := "localhost:8080"
	listener, erro := net.Dial("tcp", adr)
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer listener.Close()

	fmt.Printf("Client is connected at %s\n", adr)

	// manda cenas ao server
	mandaCenas(listener)

	// Read and display the server's response.
	response := make([]byte, 4096) // Adjust the buffer size as needed.
	n, err := listener.Read(response)
	if err != nil {
		fmt.Println("Error reading response:", err)
		os.Exit(1)
	}

	fmt.Printf("Server said: %s\n", response[:n])

	fmt.Printf("Closing connection...\n")
	defer listener.Close()
}

func mandaCenas(conn net.Conn) {
	data := []byte("AB OMB")
	_, err := conn.Write(data)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
}
