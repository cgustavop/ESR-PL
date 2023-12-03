package main

import (
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

var ErrNoFavNeighbours = errors.New("No favourite neighbour found")

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

// Maps "filePath" to the port where it will be streamed
var fileAvailable map[string]string

var bootstrapperAddr string
var activeStreams int = 0
var rpFlag bool
var nodeAddr string
var server string
var format string = ".mjpeg.avi"

func main() {
	//neighboursTable = make(map[string]neighbour)
	fileAvailable = make(map[string]string)

	//flag.StringVar(&bootstrapperAddr, "bs", "", "sets the ip address of the bootstrapper node")
	flag.StringVar(&nodeAddr, "ip", "", "sets the node ip")
	flag.StringVar(&server, "s", "", "sets the server")
	server = "./" + server + "/"
	flag.Parse()

	//go debug()
	setup()

	// à escuta de pedidos
	listener, erro := net.Listen("tcp", nodeAddr+":8081")
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer listener.Close()

	fmt.Println("Node is listening on port 8081")

	for {
		client, err := listener.Accept()
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		go handleRequest(client)
	}

	select {}

}

// Fetches Node's overlay neighbours from bootstrapper
func setup() {

	files, err := os.ReadDir(server)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		//filePath := file.Name()
		filePath := filepath.Join(server, file.Name())
		println("[setup] Added file ", filePath)
		port := strconv.Itoa(8000 + activeStreams)
		fileAvailable[filePath] = port
		activeStreams++
	}

	select {}
}

func handleRequest(client net.Conn) {
	// lê pacote
	decoder := gob.NewDecoder(client)

	var receivedData packet
	err := decoder.Decode(&receivedData)
	if err != nil {
		fmt.Println("Erro no decode da mensagem: ", err)
		return
	}

	// switch case pelo tipo (0-flood, 1-request, 2-response, 3-control)
	switch receivedData.ReqType {
	case 0:
		//flood protocol
	case 1:
		println("Stream Request received for ", receivedData.Description)
		streamFile(client, receivedData.Description, receivedData.Payload.Sender)
	case 2:
		// recebe resposta a pedido
	case 3:
		// control packets
		println("Control Packet received")
	}
}

func streamFile(clientConn net.Conn, file string, clientAddr string) {
	filePath := server + file + format
	port, ok := fileAvailable[file]
	if ok {

		encoder := gob.NewEncoder(clientConn)
		startSignal := packet{
			ReqType:     2,
			Description: port,
			Payload: payload{
				Sender: clientAddr,
			},
		}
		// Encode and send the array through the connection
		err := encoder.Encode(startSignal)
		if err != nil {
			fmt.Println("Error encoding and sending data:", err)
			return
		}
		fmt.Println("Enviei sinal a ", clientAddr)

		addr := "udp://" + clientAddr + port
		cmd := exec.Command("ffmpeg", "-stream_loop", "-1", "-i", filePath, "-pix_fmt", "yuvj422p", "-s", "640x360", "-r", "25", "-c:v", "mjpeg", "-f", "mjpeg", addr)
		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
			return
		}
		println("Sent to ", addr)

	} else {
		fmt.Println("File does not exist:", filePath)

		encoder := gob.NewEncoder(clientConn)
		startSignal := packet{
			ReqType:     2,
			Description: "404",
			Payload: payload{
				Sender: clientAddr,
			},
		}
		// Encode and send the array through the connection
		err := encoder.Encode(startSignal)
		if err != nil {
			fmt.Println("Error encoding and sending data:", err)
			return
		}
		fmt.Println("Enviei sinal de que o ficheiro não existe a ", clientAddr)
	}

}

func debug() {
	//for {
	//	fmt.Printf("[DEBUG]----------\n%d active streams\n-----------------\n", activeStreams)
	//	fmt.Printf("%v\n", fileStreamsAvailable)
	//	time.Sleep(20 * time.Second)
	//}
	for {
		fmt.Printf("[DEBUG]----------\n%d ACTIVE STREAMS \n-----------------\n", activeStreams)
		fmt.Printf("%v\n", activeStreams)
		time.Sleep(10 * time.Second)
	}
}
