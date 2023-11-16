package main

import (
	"esr/server/unicast"
	"fmt"
	"net"
	"os"
)

func main() {

	// o server deve mandar um pedido ao bootstrapper para conhecer o RP
	// apenas deve aceitar conexões do RP

	listener, erro := net.Listen("tcp", "localhost:8080")
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer listener.Close()

	fmt.Println("Server is listening on port 8080")

	for {
		client, err := listener.Accept()
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		go handleRequest(client)
	}

}

func handleRequest(conn net.Conn) {

	response := make([]byte, 4096)
	msg, err := conn.Read(response)
	if err != nil {
		fmt.Println("Error reading response:", err)
		os.Exit(1)
	}

	filePath := fmt.Sprintf("./files/%s", response[:msg])

	_, err = os.Stat(filePath)
	if err != nil {
		fmt.Println("Requested file does not exist")
		sendMsg(conn, "404")
		return
	}

	fmt.Printf("Request for %s from %s\n", filePath, conn.RemoteAddr().String())

	// ready to send file
	sendMsg(conn, "GO")
	packetsSent, UDPconn := unicast.SendFile(conn, filePath)
	fmt.Println("Ended transfer")
	sendMsg(conn, "EOF")

	//listen for number of packets received
	response = make([]byte, 4096)
	msg, err = conn.Read(response)
	if err != nil {
		fmt.Println("Error reading response:", err)
		os.Exit(1)
	}

	UDPconn.Close()

	fmt.Println("Packets sent:", packetsSent)
	packetsReceived := fmt.Sprintf("%s", response[:msg])
	fmt.Printf("The client received %s packets\n", packetsReceived)
}

func sendMsg(conn net.Conn, msg string) {
	data := []byte(msg)
	_, err := conn.Write(data)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
}
