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
	fmt.Println("Enviei pedido de stream a ", overlayAddr)

	// info pacote (guarda na tabela)
	var receivedData packet
	decoder := gob.NewDecoder(sourceConn)
	err = decoder.Decode(&receivedData)
	if err != nil {
		fmt.Println("Erro no decode da mensagem: ", err)
		return
	}

	println(receivedData.Description)
	sourceUDPaddr := nodeAddr + ":" + receivedData.Description
	cmd := exec.Command("ffplay", "udp://"+sourceUDPaddr)
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	// confirmação de stream
	var confirmation packet
	decoder = gob.NewDecoder(sourceConn)
	err = decoder.Decode(&confirmation)
	if err != nil {
		fmt.Println("Erro no decode da mensagem: ", err)
		return
	}

	if confirmation.Description == "404" {
		fmt.Println("FICHEIRO NÃO EXISTE")
		err = cmd.Process.Kill()
		if err != nil {
			fmt.Println("Error killing process:", err)
			return
		}

		fmt.Println("Command terminated")
	}

}
