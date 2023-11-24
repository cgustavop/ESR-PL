package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"net"
)

// pacote
type packet struct {
	ReqType int
	Payload string
}

func main() {
	var file string
	flag.StringVar(&file, "f", "", "requests a stream for the given file")

	flag.Parse()

	if file != "" {
		// corre bootstrapper em segundo plano
		requestStream("localhost", file)
	}
}

func requestStream(neighbourIP string, filePath string) {
	request := packet{
		ReqType: 1,
		Payload: filePath,
	}

	// connect TCP (ip)
	neighbourConn, erro := net.Dial("tcp", "localhost:8081")
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer neighbourConn.Close()
	// espera resposta TCP
	encoder := gob.NewEncoder(neighbourConn)

	// Encode and send the array through the connection
	err := encoder.Encode(request)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
		return
	}
	fmt.Println("Enviei pedido a ", neighbourIP)

	// info pacote (guarda na tabela)
	var receivedData packet
	decoder := gob.NewDecoder(neighbourConn)
	err = decoder.Decode(&receivedData)
	if err != nil {
		fmt.Println("Erro no decode da mensagem: ", err)
		return
	}

	println(receivedData.Payload)

	// junta-se ao grupo multicast do vizinho
	multicastAddr := neighbourIP + ":" + receivedData.Payload
	joinMulticastStream(multicastAddr)
}

func joinMulticastStream(multicastAddr string) {

	// Resolve multicast address
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8888")
	if err != nil {
		fmt.Println("Error resolving address:", err)
		return
	}

	// Join the multicast group
	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		fmt.Println("Error joining multicast:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Multicast server joined group", "127.0.0.1:8888")

	// Buffer for incoming data
	buffer := make([]byte, 1024)

	for {
		// Read data from the connection
		n, src, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			continue
		}
		fmt.Printf("Received %d bytes from %s: %s\n", n, src, string(buffer[:n]))
	}
}
