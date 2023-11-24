package main

import (
	"encoding/gob"
	"errors"
	"esr/node/bootstrapper"
	"flag"
	"fmt"
	"net"
	"strconv"
	"time"
)

var ErrNoFavNeighbours = errors.New("No favourite neighbour found")

var rpFlag bool

type neighbour struct {
	flag    int
	latency int
	loss    int
}

// pacote
type packet struct {
	ReqType int
	Payload string
}

// Maps the neighbour's IP to it's related info (type flag, latency, loss)
var neighboursTable map[string]neighbour

// Maps "filePath" to the multicast group Port where the file is being streamed
var fileStreamsAvailable map[string]int

// hard-coded
var bootrapperAddr string = "localhost:8080"

func main() {
	neighboursTable = make(map[string]neighbour)
	fileStreamsAvailable = make(map[string]int)

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
	if rpFlag {
		//go streamFile("8888", "teste")
	}

	// faz pedido ao bootstrapper e guarda vizinhos
	setup()

	if !rpFlag {
		requestStream("teste")
	} else {

		// à escuta de pedidos de outros nodos
		listener, erro := net.Listen("tcp", ":8081")
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
	}

	select {}
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
		println("Flood Package received")
	case 1:
		println("Stream Request received")
		//streamRequest(client, receivedData.payload)
		streamFile(client, "8888", "teste")
		// abre multicast e faz stream
	case 2:
		// recebe resposta a pedido
	case 3:
		// control packets
		println("Control Packet received")
	}
}

func streamRequest(client net.Conn, filePath string) {

	println("Pedido de ficheiro", filePath)
	// check table
	streamPort, ok := fileStreamsAvailable[filePath]
	if !ok {
		// if not in table sendRequest
		requestStream(filePath)
	} else {
		// invite client to join multicast group
		redirectClient(client, streamPort, filePath)
	}
}

func streamFile(client net.Conn, port string, filePath string) {
	//ip := client.RemoteAddr().(*net.TCPAddr).IP.String()
	//multicastAddr, err := net.ResolveUDPAddr("udp", ip+":"+port)
	multicastAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+port)
	if err != nil {
		fmt.Println("Error resolving address:", err)
		return
	}

	// Create a UDP connection
	conn, err := net.DialUDP("udp", nil, multicastAddr)
	if err != nil {
		fmt.Println("Error dialing:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected to multicast group", multicastAddr)
	fileStreamsAvailable[filePath], _ = strconv.Atoi(port)

	handshake(client, filePath, port)

	hello := "hello, world"
	for {
		println(hello)
		conn.Write([]byte(hello))
		time.Sleep(1 * time.Second)
	}
}

func handshake(client net.Conn, filePath string, port string) {
	// this should fetch the file details and put it onto the response packet
	// fetchFileInfo(filePath)

	response := packet{
		ReqType: 2,
		Payload: port,
	}

	encoder := gob.NewEncoder(client)

	// Encode and send the array through the connection
	err := encoder.Encode(response)
	if err != nil {
		fmt.Println("[handshake] Error encoding and sending data:", err)
		return
	}
	fmt.Println("Enviei informação sobre a stream a ", client.RemoteAddr().String())
}

func redirectClient(client net.Conn, port int, filePath string) {
	request := packet{
		ReqType: 2,
		Payload: string(rune(port)),
	}

	encoder := gob.NewEncoder(client)

	// Encode and send the array through the connection
	err := encoder.Encode(request)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
		return
	}
	fmt.Println("Enviei convite para juntar ao multicast a ", client.RemoteAddr().String())
}

func requestStream(filePath string) {
	request := packet{
		ReqType: 1,
		Payload: filePath,
	}
	// envia aos vizinho preferido
	// getFavNeighbour() -> ip
	neighbourIP, err := getFavNeighbour()
	if err != nil {
		println("Erro: ", err)
		return
	}

	// connect TCP (ip)
	neighbourConn, erro := net.Dial("tcp", neighbourIP+":8081")
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer neighbourConn.Close()
	// espera resposta TCP
	encoder := gob.NewEncoder(neighbourConn)

	// Encode and send the array through the connection
	err = encoder.Encode(request)
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
	addr, err := net.ResolveUDPAddr("udp", multicastAddr)
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

	fmt.Println("Multicast server joined group", "localhost:8888")

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

func addNeighbours(array []string) {
	for _, n := range array {
		neighboursTable[n] = neighbour{
			flag:    1,
			latency: 0,
			loss:    0,
		}
	}
}

func getFavNeighbour() (string, error) {
	for ip, n := range neighboursTable {
		if n.flag == 1 {
			return ip, nil
		}
	}
	return "", ErrNoFavNeighbours
}
