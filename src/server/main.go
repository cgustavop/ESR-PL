package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	// abrir a socket para receber pedidos
	listener, erro := net.Listen("tcp", "localhost:8080")
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer listener.Close()

	fmt.Println("Server is listening on port 8080")

	// loop para receber pedidos
	for {
		// aceitar um pedido recebido na socket
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		// Handle client connection in a goroutine
		go handleRequest(conn)
	}
}

func handleRequest(conn net.Conn) {
	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err != nil {
		fmt.Println("Error reading response:", err)
		os.Exit(1)
	}

	fmt.Printf("Client said: %s\n", response[:n])

	data := []byte("ok")
	_, err = conn.Write(data)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	defer conn.Close()

}
