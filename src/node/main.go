package main

import (
	"encoding/gob"
	"esr/node/bootstrapper"
	"flag"
	"fmt"
	"net"
	"os"
)

/*
pedido bootrapper
ficar à escuta no TCP
	handler pedido de stream
gestor de vizinhos
	map[filePath string]map[ip string]()
	ou
	map[ip_vizinho string]bool on
faz pedido de stream
video stream player
*/

var rpFlag bool

type neighbour struct {
	flag    int
	latency int
	loss    int
}

var neighboursTable map[string]neighbour

// hard-coded
var bootrapperAddr string = "localhost:8080"

func main() {
	neighboursTable = make(map[string]neighbour)

	bsFlag := flag.Bool("bs", false, "sets the node as the overlay's Bootstrapper")
	flag.BoolVar(&rpFlag, "rp", false, "sets the node as the overlay's Rendezvous Point")
	//filePath := flag.String("stream", "", "requests a stream for the given file")

	var filePath string
	teste := "teste.png" // ISTO É SÓ PARA TESTAR
	flag.StringVar(&filePath, "stream", teste, "requests a stream for the given file")

	flag.Parse()

	if *bsFlag {
		// corre bootstrapper em segundo plano
		go bootstrapper.Run()
	}

	// faz pedido ao bootstrapper e guarda vizinhos
	setup()

	select {}
	/*
		if filePath != "" {
			// go requestStream(filePath)
		}

		listener, erro := net.Listen("tcp", "localhost:8081")
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
	*/
}

// Fetches Node's overlay neighbours from bootstrapper
func setup() {
	conn, erro := net.Dial("tcp", bootrapperAddr)
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer conn.Close()

	data := []byte("RESOLVE")
	_, err := conn.Write(data)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	decoder := gob.NewDecoder(conn)

	var neighboursArray []string
	err = decoder.Decode(&neighboursArray)
	if err != nil {
		fmt.Println("Erro a receber vizinhos: ", err)
		return
	}

	addNeighbours(neighboursArray)

	fmt.Println("Recebi os vizinhos ", neighboursArray)
}

func addNeighbours(array []string) {
	for _, n := range array {
		neighboursTable[n] = neighbour{
			flag:    0,
			latency: 0,
			loss:    0,
		}
	}
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
	buffer := make([]byte, 1472)

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
