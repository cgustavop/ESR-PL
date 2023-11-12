package main

import (
	"fmt"
	"net"
	"os"
	"server/unicast"
)

func main() {
	// abrir a socket UDP para receber pedidos
	udpAddr, err := net.ResolveUDPAddr("udp", "localhost:8080")

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	listener, erro := net.ListenUDP("udp", udpAddr)
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer listener.Close()

	fmt.Println("Server is listening on port 8080")

	for {
		var buf [512]byte
		msg, addr, err := listener.ReadFromUDP(buf[0:])
		if err != nil {
			fmt.Println(err)
			return
		}
		filePath := string(buf[:msg])
		fp := fmt.Sprintf("./files/%s", filePath)

		handleRequest(listener, addr, fp) // go X arranca uma nova thread que executa a função X
		// Write back the message over UPD

	}
}

func handleRequest(conn *net.UDPConn, client *net.UDPAddr, filePath string) {
	fmt.Printf("Request for %s from %s\n", filePath, client.IP.String())
	//data := []byte("ACK")
	//conn.WriteToUDP(data, client)
	unicast.SendFile(conn, client, filePath)
}
