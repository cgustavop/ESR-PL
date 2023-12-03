package main

import (
	"encoding/gob"
	"errors"
	"esr/node/bootstrapper"
	"flag"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"
	"time"
)

var ErrNoFavNeighbours = errors.New("No favourite neighbour found")

// flood
type Flood struct {
	HourIn        time.Time
	AllNodes      []int
	BacktraceSize int
	TotalTime     int //ms
}

type neighbour struct {
	flag    int
	latency int
	loss    int
}

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

// Maps the neighbour's IP to it's related info (type flag, latency, loss)
var neighboursTable map[string]neighbour

// Maps "filePath" to the multicast group Port where the file is being streamed
var fileStreamsAvailable map[string]chan []byte

// hard-coded
var bootstrapperAddr string
var activeStreams int = 0
var rpFlag bool
var nodeAddr string
var serverAddr string

func main() {
	neighboursTable = make(map[string]neighbour)
	fileStreamsAvailable = make(map[string]chan []byte)

	flag.StringVar(&bootstrapperAddr, "bs", "", "sets the ip address of the bootstrapper node")
	var startFlood bool
	flag.BoolVar(&startFlood, "f", false, "starts flood")
	flag.BoolVar(&rpFlag, "rp", false, "sets the node as the overlay's Rendezvous Point")
	flag.StringVar(&serverAddr, "s", "", "sets the server wich the RP has connection to")
	flag.StringVar(&nodeAddr, "ip", "", "sets the node ip")

	//filePath := flag.String("stream", "", "requests a stream for the given file")

	var filePath string
	flag.StringVar(&filePath, "stream", "", "requests a stream for the given file")

	flag.Parse()

	if bootstrapperAddr == "" {
		// corre bootstrapper em segundo plano
		go bootstrapper.Run(nodeAddr)
		bootstrapperAddr = nodeAddr
	}

	go debug()
	// faz pedido ao bootstrapper e guarda vizinhos
	setup()

	if rpFlag && serverAddr != "" {
		neighboursTable[serverAddr] = neighbour{
			flag:    4,
			latency: 0,
			loss:    0,
		}
	}

	if startFlood {
		go flood()
	}
	if filePath != "" {
		go getStream(filePath)
	}

	// à escuta de pedidos de outros nodos
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
	conn, erro := net.Dial("tcp", bootstrapperAddr+":8080")
	if erro != nil {
		fmt.Println("Error:", erro)
		ducktape()
		return
	}
	defer conn.Close()

	data := []byte(nodeAddr)
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

func ducktape() {
	n, _ := bootstrapper.GetNeighbours2(nodeAddr, bootstrapper.LoadTree2())
	addNeighbours(n)
}

func debug() {
	//for {
	//	fmt.Printf("[DEBUG]----------\n%d active streams\n-----------------\n", activeStreams)
	//	fmt.Printf("%v\n", fileStreamsAvailable)
	//	time.Sleep(20 * time.Second)
	//}
	for {
		fmt.Printf("[DEBUG]----------\n%d NEIGHBOUR STATUS \n-----------------\n", len(neighboursTable))
		fmt.Printf("%v\n", neighboursTable)
		time.Sleep(10 * time.Second)
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
		println("Flood Package received")
		if rpFlag {
			floodPivotPoint(receivedData.Payload)
		} else {
			floodRetransmit(receivedData.Payload)
		}
	case 1:
		println("Stream Request received")
		streamRequest(client, receivedData)
		//streamFile(client, "8888", "teste")
		// abre multicast e faz stream
	case 2:
		// recebe resposta a pedido
	case 3:
		// control packets
		println("Control Packet received")
	}
}

func streamRequest(client net.Conn, receivedPkt packet) {
	filePath := receivedPkt.Description

	println("Pedido de ficheiro", filePath)
	// check table
	dataChannel, ok := fileStreamsAvailable[filePath]
	if !ok {
		// if not in table sendRequest
		println("não tenho o ficheiro: ", filePath)

		// Shared channel to distribute received data to multiple goroutines
		dataChannel := make(chan []byte)
		fileStreamsAvailable[filePath] = dataChannel
		go requestStream(client, filePath, receivedPkt.Payload.Sender, dataChannel)
		distributeData(client, receivedPkt.Payload.Sender, dataChannel)

	} else {
		// invite client to join multicast group
		println("Stream já existe, redirecionando cliente...")
		//redirectClient(client, strconv.Itoa(streamPort), filePath)
		distributeData(client, receivedPkt.Payload.Sender, dataChannel)
	}
}

func handshake(client net.Conn, filePath string, port string) {
	// this should fetch the file details and put it onto the response packet
	// fetchFileInfo(filePath)

	response := packet{
		ReqType:     2,
		Description: port,
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

func redirectClient(client net.Conn, port string, filePath string) {
	request := packet{
		ReqType:     2,
		Description: port,
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

// func requestStream(clientConn net.Conn, filePath string, clientAddr string) {
// 	request := packet{
// 		ReqType:     1,
// 		Description: filePath,
// 		Payload: payload{
// 			Sender: nodeAddr,
// 		},
// 	}
// 	// envia aos vizinho preferido
// 	// getFavNeighbour() -> ip
// 	neighbourIP, err := getFavNeighbour()
// 	if err != nil {
// 		println("Erro: ", err)
// 		return
// 	}

// 	// connect TCP (ip)
// 	sourceConn, erro := net.Dial("tcp", neighbourIP+":8081")
// 	if erro != nil {
// 		fmt.Println("Error:", erro)
// 		return
// 	}
// 	defer sourceConn.Close()
// 	// espera resposta TCP
// 	encoder := gob.NewEncoder(sourceConn)

// 	// Encode and send the array through the connection
// 	err = encoder.Encode(request)
// 	if err != nil {
// 		fmt.Println("Error encoding and sending data:", err)
// 		return
// 	}
// 	fmt.Println("Enviei pedido a ", neighbourIP)

// 	// info pacote (guarda na tabela)
// 	var receivedData packet
// 	decoder := gob.NewDecoder(sourceConn)
// 	err = decoder.Decode(&receivedData)
// 	if err != nil {
// 		fmt.Println("Erro no decode da mensagem: ", err)
// 		return
// 	}

// 	println(receivedData.Description)
// 	sourceUDPaddr := receivedData.Payload.Sender + ":" + receivedData.Description

// 	if receivedData.Description == "404" {
// 		// avisa cliente/nodo atrás
// 		port := "404"
// 		encoder := gob.NewEncoder(clientConn)
// 		noFileSignal := packet{
// 			ReqType:     2,
// 			Description: port,
// 			Payload: payload{
// 				Sender: nodeAddr,
// 			},
// 		}
// 		// Encode and send the array through the connection
// 		err = encoder.Encode(noFileSignal)
// 		if err != nil {
// 			fmt.Println("Error encoding and sending data:", err)
// 			return
// 		}
// 		fmt.Println("FICHEIRO NÃO EXISTE: Enviei aviso a ", clientAddr)
// 	} else {
// 		//pronto para receber
// 		// avisa cliente/nodo atrás
// 		activeStreams++
// 		portCalc := 8000 + activeStreams
// 		port := strconv.Itoa(portCalc)
// 		encoder := gob.NewEncoder(clientConn)
// 		startSignal := packet{
// 			ReqType:     2,
// 			Description: port,
// 			Payload: payload{
// 				Sender: nodeAddr,
// 			},
// 		}
// 		// Encode and send the array through the connection
// 		err = encoder.Encode(startSignal)
// 		if err != nil {
// 			fmt.Println("Error encoding and sending data:", err)
// 			return
// 		}
// 		fmt.Println("Enviei sinal a ", clientAddr)
// 		// abre porta de leitura do servidor
// 		// Resolve UDP address Server
// 		srcAddr, err := net.ResolveUDPAddr("udp", sourceUDPaddr)
// 		if err != nil {
// 			log.Fatal(err)
// 		}

// 		// Listen for UDP connections server
// 		src, err := net.ListenUDP("udp", srcAddr)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer src.Close()
// 		// abre porta de escrita para o cliente
// 		redirAddr, err := net.ResolveUDPAddr("udp", nodeAddr+":"+port)
// 		if err != nil {
// 			log.Fatal(err)
// 		}

// 		// Dial UDP for redirection client
// 		redirConn, err := net.DialUDP("udp", nil, redirAddr)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		defer redirConn.Close()

// 		// Shared channel to distribute received data to multiple goroutines
// 		dataChannel := make(chan []byte)

// 		// Start a goroutine to distribute data among multiple consumer goroutines
// 		go distributeData(dataChannel)

// 		// Buffer to store incoming data
// 		buf := make([]byte, 2048)
// 		time.Sleep(5 * time.Second)
// 		// Continuously read data from the connection and redirect it
// 		//buffer := make([]byte, 1024)
// 		for {
// 			n, _, err := src.ReadFromUDP(buf)
// 			if err != nil {
// 				log.Fatal(err)
// 			}
// 			println("Recieved from source ", sourceUDPaddr)

// 			//receivedData := make([]byte, n)
// 			//copy(receivedData, buffer[:n])

// 			// Send the received data to the channel for distribution
// 			//dataChannel <- receivedData

// 			// Redirect data
// 			_, err = redirConn.Write(buf[:n])
// 			if err != nil {
// 				log.Fatal(err)
// 			}
// 			println("Sent to client ", nodeAddr+":"+port)
// 		}
// 	}
// }

func requestStream(clientConn net.Conn, filePath string, clientAddr string, dataChannel chan []byte) {
	request := packet{
		ReqType:     1,
		Description: filePath,
		Payload: payload{
			Sender: nodeAddr,
		},
	}
	// envia aos vizinho preferido
	// getFavNeighbour() -> ip
	neighbourIP, err := getFavNeighbour()
	if err != nil {
		println("Erro: ", err)
		return
	}

	// connect TCP (ip)
	sourceConn, erro := net.Dial("tcp", neighbourIP+":8081")
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer sourceConn.Close()
	// espera resposta TCP
	encoder := gob.NewEncoder(sourceConn)

	// Encode and send the array through the connection
	err = encoder.Encode(request)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
		return
	}
	fmt.Println("Enviei pedido a ", neighbourIP)

	// info pacote (guarda na tabela)
	var receivedData packet
	decoder := gob.NewDecoder(sourceConn)
	err = decoder.Decode(&receivedData)
	if err != nil {
		fmt.Println("Erro no decode da mensagem: ", err)
		return
	}

	println(receivedData.Description)
	sourceUDPaddr := receivedData.Payload.Sender + ":" + receivedData.Description

	if receivedData.Description == "404" {
		// avisa cliente/nodo atrás
		port := "404"
		encoder := gob.NewEncoder(clientConn)
		noFileSignal := packet{
			ReqType:     2,
			Description: port,
			Payload: payload{
				Sender: nodeAddr,
			},
		}
		// Encode and send the array through the connection
		err = encoder.Encode(noFileSignal)
		if err != nil {
			fmt.Println("Error encoding and sending data:", err)
			return
		}
		fmt.Println("FICHEIRO NÃO EXISTE: Enviei aviso a ", clientAddr)
		close(dataChannel)
		delete(fileStreamsAvailable, filePath)
	} else {
		//pronto para receber
		// abre porta de leitura do servidor
		// Resolve UDP address Server
		srcAddr, err := net.ResolveUDPAddr("udp", sourceUDPaddr)
		if err != nil {
			log.Fatal(err)
		}

		// Listen for UDP connections server
		src, err := net.ListenUDP("udp", srcAddr)
		if err != nil {
			log.Fatal(err)
		}
		defer src.Close()

		// Buffer to store incoming data
		buf := make([]byte, 2048)
		time.Sleep(5 * time.Second)
		// Continuously read data from the connection and redirect it
		buffer := make([]byte, 1024)
		for {
			n, _, err := src.ReadFromUDP(buf)
			if err != nil {
				log.Fatal(err)
			}
			println("Recieved from source ", sourceUDPaddr)

			receivedData := make([]byte, n)
			copy(receivedData, buffer[:n])

			// Send the received data to the channel for distribution
			dataChannel <- receivedData
		}
	}
}

func distributeData(clientConn net.Conn, clientAddr string, dataChannel <-chan []byte) {
	select {
	case _, ok := <-dataChannel:
		if !ok {
			fmt.Println("Channel closed before receiving anything. Ficheiro não existe.")
			return
		}
		activeStreams++
		portCalc := 8000 + activeStreams
		port := strconv.Itoa(portCalc)
		encoder := gob.NewEncoder(clientConn)
		startSignal := packet{
			ReqType:     2,
			Description: port,
			Payload: payload{
				Sender: nodeAddr,
			},
		}
		// Encode and send the array through the connection
		err := encoder.Encode(startSignal)
		if err != nil {
			fmt.Println("Error encoding and sending data:", err)
			return
		}
		fmt.Println("Enviei sinal a ", clientAddr)

		//abre porta de escrita para o cliente
		redirAddr, err := net.ResolveUDPAddr("udp", nodeAddr+":"+port)
		if err != nil {
			log.Fatal(err)
		}

		// Dial UDP for redirection client
		redirConn, err := net.DialUDP("udp", nil, redirAddr)
		if err != nil {
			log.Fatal(err)
		}
		defer redirConn.Close()

		for {
			// Receive data from the channel
			data := <-dataChannel

			// Redirect data to client
			_, err = redirConn.Write(data)
			if err != nil {
				log.Fatal(err)
			}
			println("Sent to client ", nodeAddr+":"+port)
		}
	default:
		fmt.Println("[distributeData] No data received. ", clientAddr)
	}

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

	fmt.Println("Multicast server joined group", multicastAddr)

	maxDatagramSize := 8192
	conn.SetReadBuffer(maxDatagramSize)

	// Loop forever reading from the socket
	for {
		buffer := make([]byte, maxDatagramSize)
		numBytes, src, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			continue
		}

		fmt.Printf("Received %d bytes from %s: %s\n", numBytes, src, string(buffer[:numBytes]))
	}
}

func addNeighbours(array []string) {
	for _, n := range array {
		neighboursTable[n] = neighbour{
			flag:    0, //inicialmente sem ligação ao RP
			latency: 0,
			loss:    0,
		}
	}
}

func getFavNeighbour() (string, error) {
	var best int = 0
	var bestip string

	for ip, n := range neighboursTable {
		if n.flag > best {
			best = n.flag
			bestip = ip
		}
	}
	if best == 0 {
		return "", ErrNoFavNeighbours
	}
	return bestip, nil
}

func floodRetransmit(receivedPkt payload) {
	ipNeighbour := receivedPkt.Sender
	println("[flood debug] tempo total:", receivedPkt.TotalTime)
	// recebe pacote se vier com TotalTime > 0 então envia para o ip no backtrace
	if receivedPkt.TotalTime >= 0 {
		saveFavNeighbour(receivedPkt)
		sendBack(receivedPkt)
	} else {
		a := receivedPkt.AllNodes
		a = append(a, nodeAddr)
		btSize := receivedPkt.BacktraceSize + 1

		response := payload{
			Sender:        nodeAddr,
			HourIn:        receivedPkt.HourIn,
			AllNodes:      a,
			BacktraceSize: btSize,
			TotalTime:     receivedPkt.TotalTime,
		}

		retransmitPkt := packet{
			ReqType: 0,
			Payload: response,
		}

		sendToNeighbours(ipNeighbour, retransmitPkt)
	}

}

func saveFavNeighbour(pkt payload) {
	flagVal := 2

	// se quem enviou for o rp então guarda vizinho com a flag 3
	fmt.Printf("bts: %v\n%v\nlength all nodes: %v\n", pkt.BacktraceSize, pkt.AllNodes, len(pkt.AllNodes))
	if len(pkt.AllNodes) == pkt.BacktraceSize+2 {
		flagVal = 3
	}

	n := neighbour{
		flag:    flagVal,
		latency: pkt.TotalTime,
		loss:    0,
	}
	sender := pkt.Sender

	// compara latência antes de guardar vizinho
	if changeNeighbourMetric(n, sender) {
		neighboursTable[sender] = n
		println("Guardei o vizinho ", sender)
	}
}

func sendBack(pkt payload) {
	fmt.Printf("%v\n", pkt)
	btSize := pkt.BacktraceSize - 1

	if btSize == -1 {
		return
	}

	response := payload{
		Sender:        nodeAddr,
		HourIn:        pkt.HourIn,
		AllNodes:      pkt.AllNodes,
		BacktraceSize: btSize,
		TotalTime:     pkt.TotalTime,
	}

	neighbourIp := pkt.AllNodes[pkt.BacktraceSize-1]

	println(neighbourIp)
	retransmitPkt := packet{
		ReqType: 0,
		Payload: response,
	}

	neighbourConn, erro := net.Dial("tcp", neighbourIp+":8081")
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer neighbourConn.Close()

	encoder := gob.NewEncoder(neighbourConn)

	// Encode and send the array through the connection
	err := encoder.Encode(retransmitPkt)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
		return
	}
	fmt.Println("Enviei flood packet a ", neighbourIp)
}

func changeNeighbourMetric(info neighbour, ip string) bool {
	current, ok := neighboursTable[ip]

	if ok && current.latency > info.latency || current.flag == 0 {
		return true
	}
	return false
}

func floodPivotPoint(receivedPkt payload) {
	a := receivedPkt.AllNodes
	a = append(a, nodeAddr)

	currentTime := time.Now()
	duration := int(currentTime.Sub(receivedPkt.HourIn).Milliseconds())

	response := payload{
		Sender:        nodeAddr,
		HourIn:        receivedPkt.HourIn,
		AllNodes:      a,
		BacktraceSize: receivedPkt.BacktraceSize,
		TotalTime:     duration,
	}

	retransmitPkt := packet{
		ReqType: 0,
		Payload: response,
	}

	ip := receivedPkt.Sender

	neighbourConn, erro := net.Dial("tcp", ip+":8081")
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer neighbourConn.Close()

	encoder := gob.NewEncoder(neighbourConn)

	// Encode and send the array through the connection
	err := encoder.Encode(retransmitPkt)
	if err != nil {
		fmt.Println("[rp] Error encoding and sending data:", err)
		return
	}
	fmt.Println("Respondi ao flood de ", ip)
}

func sendToNeighbours(ipNeighbour string, pkt packet) {
	//envia struct packet com payload e reqtype a 0
	//for com numero de vizinhos em que faço sendtoneighbour em loop

	for n := range neighboursTable {
		if n != ipNeighbour {
			neighbourConn, erro := net.Dial("tcp", n+":8081")
			if erro != nil {
				fmt.Println("Error:", erro)
				return
			}
			defer neighbourConn.Close()

			encoder := gob.NewEncoder(neighbourConn)

			// Encode and send the array through the connection
			err := encoder.Encode(pkt)
			if err != nil {
				fmt.Println("[handshake] Error encoding and sending data:", err)
				return
			}
			fmt.Println("Enviei flood packet a ", n)
		}
	}
}

func flood() {
	var nodes []string
	nodes = append(nodes, nodeAddr)

	pl := payload{
		Sender:        nodeAddr,
		HourIn:        time.Now(),
		AllNodes:      nodes,
		BacktraceSize: 0,
		TotalTime:     -1,
	}

	pkt := packet{
		ReqType: 0,
		Payload: pl,
	}

	sendToNeighbours("", pkt)
}

func getStream(filePath string) {

	request := packet{
		ReqType:     1,
		Description: filePath,
		Payload: payload{
			Sender: nodeAddr,
		},
	}
	// envia aos vizinho preferido
	// getFavNeighbour() -> ip
	neighbourIP, err := getFavNeighbour()
	if err != nil {
		println("Erro: ", err)
		return
	}

	// connect TCP (ip)
	sourceConn, erro := net.Dial("tcp", neighbourIP+":8081")
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer sourceConn.Close()
	// espera resposta TCP
	encoder := gob.NewEncoder(sourceConn)

	// Encode and send the array through the connection
	err = encoder.Encode(request)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
		return
	}
	fmt.Println("Enviei pedido de stream a ", neighbourIP)

	// info pacote (guarda na tabela)
	var receivedData packet
	decoder := gob.NewDecoder(sourceConn)
	err = decoder.Decode(&receivedData)
	if err != nil {
		fmt.Println("Erro no decode da mensagem: ", err)
		return
	}

	println(receivedData.Description)
	sourceUDPaddr := receivedData.Payload.Sender + ":" + receivedData.Description

	if receivedData.Description == "404" {
		fmt.Println("FICHEIRO NÃO EXISTE")
	} else {
		cmd := exec.Command("ffplay", "udp://"+sourceUDPaddr)
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}
}
