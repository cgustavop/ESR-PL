package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
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

func main() {

	flag.StringVar(&overlayAddr, "o", "", "sets the overlay address to send requests")
	flag.StringVar(&nodeAddr, "ip", "", "sets the node ip")
	flag.StringVar(&filePath, "stream", "", "requests a stream for the given file")
	flag.Parse()

	getStream()

}

func getStream() {

	request := packet{
		ReqType:     1,
		Description: filePath,
		Payload: payload{
			Sender: nodeAddr,
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

	// Encode and send the array through the connection
	err := encoder.Encode(request)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
		return
	}
	fmt.Println("Pedido de stream enviado a ", overlayAddr)

	// info pacote (guarda na tabela)
	var receivedData packet
	decoder := gob.NewDecoder(sourceConn)
	err = decoder.Decode(&receivedData)
	if err != nil {
		fmt.Println("Erro no decode da mensagem: ", err)
		return
	}

	sourceUDPaddr := nodeAddr + ":" + receivedData.Description
	cmd := exec.Command("ffplay", "udp://"+sourceUDPaddr)
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	terminate := make(chan struct{})

	// Handle interrupt signal (e.g., Ctrl+C) to gracefully terminate the program
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go stream(sourceUDPaddr, terminate)

	err = encoder.Encode(request)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
		return
	}
	//log.Println("Ready signal enviado a ", overlayAddr)

	// confirmação de stream
	var confirmation packet
	err = decoder.Decode(&confirmation)
	if err != nil {
		log.Println("Erro no decode da mensagem: ", err)
		return
	}

	if confirmation.Description == "404" {
		fmt.Println("Ficheiro não existe")
		<-sigCh
		close(terminate)
	} else {
		fmt.Println("Iniciando stream")
	}
	select {}
}

func stream(addr string, terminate <-chan struct{}) {
	cmd := exec.Command("ffplay", "udp://"+addr)
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	select {
	case <-terminate: // If termination signal received
		err := cmd.Process.Kill()
		if err != nil {
			fmt.Println("Error killing process:", err)
		}
		fmt.Println("Command terminated")
	}
}
