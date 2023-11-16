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
	listener, erro := net.Dial("tcp", adr)
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer listener.Close()

	fmt.Println("Client is connected at ", adr)

	// manda cenas ao server
	requestFile(listener, filePath)

	// Read server's response.
	response := make([]byte, 4096)
	n, err := listener.Read(response)
	if err != nil {
		fmt.Println("Error reading response:", err)
		os.Exit(1)
	}

	r := string(response[:n])

	if r == "GO" {
		fmt.Println("Server ready for file transfer")
		fileTransfer(listener, filePath)
		fmt.Println("Received file successfully")
	} else {
		fmt.Println("The file was not found")
	}

	fmt.Println("Closing connection...")
	defer listener.Close()
}

func requestFile(conn net.Conn, filePath string) {
	data := []byte(filePath)
	_, err := conn.Write(data)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
}

func fileTransfer(connTCP net.Conn, filePath string) {
	var packetCount int
	packetChan := make(chan struct{})
	// abre conn UDP

	addr, err := net.ResolveUDPAddr("udp", connTCP.RemoteAddr().String())
	addr.Port = 8081
	listenerUDP, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}
	defer listenerUDP.Close()
	fmt.Println("Connected at ", addr)
	// recebe pacotes
	go receivePackets(listenerUDP, filePath, packetChan, &packetCount)

	//espera EOF
	response := make([]byte, 4096)
	msg, err := connTCP.Read(response)
	r := string(response[:msg])
	if err != nil {
		fmt.Println("Error reading response:", err)
		os.Exit(1)
	}

	//devolve numero de pacotes recebidos
	if r == "EOF" {
		close(packetChan)
		fmt.Println(packetCount)
		data := []byte(fmt.Sprintf("%d", packetCount))
		_, err := connTCP.Write(data)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
	}

}

func receivePackets(listener *net.UDPConn, filePath string, packetChan chan struct{}, packetCount *int) {
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	fmt.Println("Ready for file transfer")

	// Set buffer size
	buffer := make([]byte, 1024)

	// Loop to receive and write packets to the file
	for {
		select {
		case <-packetChan:
			fmt.Println("Stopping goroutine...")
			return
		default:
			// Receive a packet from the sender
			n, err := listener.Read(buffer)
			if err != nil {
				fmt.Println("Error receiving data:", err)
				return
			}

			// Write the received data to the file
			_, err = file.Write(buffer[:n])
			if err != nil {
				fmt.Println("Error writing to file:", err)
				return
			}
			*packetCount++
		}

	}
}
