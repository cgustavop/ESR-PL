package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"net"
	"os/exec"
	"time"
)

// pacote
type packet struct {
	ReqType     int
	Description string
	Payload     payload
}

type payload struct {
	Sender        string
	HourIn        time.Time
	AllNodes      []string
	BacktraceSize int
	TotalTime     int //ms
}

var nodeAddr string
var overlayAddr string
var filePath string
var port string = "8000"

func main() {

	flag.StringVar(&overlayAddr, "o", "", "sets the overlay address to send requests")
	flag.StringVar(&nodeAddr, "ip", "", "sets the node ip")
	flag.StringVar(&filePath, "stream", "", "requests a stream for the given file")
	flag.Parse()

	go stream(nodeAddr + ":" + port)

	getStream()
	select {}
}

func getStream() {

	request := packet{
		ReqType:     1,
		Description: filePath,
		Payload: payload{
			Sender: nodeAddr + ":" + port,
		},
	}

	// connect TCP (ip)
	sourceConn, erro := net.Dial("tcp", overlayAddr+":8081")
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer sourceConn.Close()
	// espera resposta TCP
	encoder := gob.NewEncoder(sourceConn)

	// Encode and send the request through the connection
	err := encoder.Encode(request)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
		return
	}
	fmt.Println("Pedido de stream enviado a ", overlayAddr)

	// info pacote (guarda na tabela)
	var confirmation packet
	decoder := gob.NewDecoder(sourceConn)
	err = decoder.Decode(&confirmation)
	if err != nil {
		log.Println("Erro no decode da mensagem: ", err)
		return
	}

	if confirmation.Description == "404" {
		fmt.Println("Ficheiro não existe")
	} else if confirmation.Description == "500" {
		fmt.Println("Falha interna do Overlay")
	} else {
		fmt.Println("Iniciando stream")
	}
}

// func stream(addr string, terminate <-chan struct{}) {
// 	cmd := exec.Command("ffplay", "udp://"+addr)
// 	err := cmd.Run()
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	select {
// 	case <-terminate: // If termination signal received
// 		err := cmd.Process.Kill()
// 		if err != nil {
// 			fmt.Println("Error killing process:", err)
// 		}
// 		fmt.Println("Command terminated")
// 	}
// }

func stream(addr string) {
	cmd := exec.Command("ffplay", "udp://"+addr)
	err := cmd.Run()
	if err != nil {
		log.Println(err)
		return
	}
}
