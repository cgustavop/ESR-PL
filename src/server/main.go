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
	TotalTime     float64 //ms
}

// Maps "filePath" to the port where it will be streamed
var fileAvailable map[string]string

var bootstrapperAddr string
var activeStreams int = 0
var rpFlag bool
var nodeAddr string
var server string
var format string = ".mjpeg"

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

	go monitor()

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

		filePath := file.Name()
		port := strconv.Itoa(8000 + activeStreams)
		_, ok := fileAvailable[filePath]
		if !ok {
			println("[setup] Added file ", filePath)
			fileAvailable[filePath] = port
			activeStreams++
		}

	}
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
		// control packets
		println("Control Packet received")
		sendFileList(client)
	}
}

func sendFileList(clientConn net.Conn) {
	fileList := []string{}
	for f, _ := range fileAvailable {
		fileList = append(fileList, f)
	}
	encoder := gob.NewEncoder(clientConn)
	err := encoder.Encode(fileList)
	if err != nil {
		log.Printf("Erro a enviar listagem de ficheiros ao RP")
		return
	}
}

func streamFile(clientConn net.Conn, file string, clientAddr string) {
	port, ok := fileAvailable[file+format]
	filePath := server + "/" + file + format
	if ok {
		go stream(filePath, port, clientAddr)
		time.Sleep(2 * time.Second)
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

func stream(filePath string, port string, clientAddr string) {
	println("Começando stream de ", filePath)
	addr := "udp://" + clientAddr + ":" + port
	println(addr)
	cmd := exec.Command("ffmpeg", "-stream_loop", "-1", "-i", filePath, "-pix_fmt", "yuvj422p", "-s", "640x360", "-r", "25", "-c:v", "mjpeg", "-f", "mjpeg", addr)
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
		return
	}
}

func debug() {
	for {
		fmt.Printf("[DEBUG]----------\n%d ACTIVE STREAMS \n-----------------\n", activeStreams)
		fmt.Printf("%v\n", activeStreams)
		time.Sleep(10 * time.Second)
	}
}

func monitor() {
	println("Monitoring files...")
	time.Sleep(10 * time.Second)
	setup()
}
