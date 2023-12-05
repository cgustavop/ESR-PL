package main

import (
	"encoding/gob"
	"errors"
	"esr/node/bootstrapper"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
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
var streamViewers map[string]int

// hard-coded
var bootstrapperAddr string
var activeStreams int = 0
var rpFlag bool
var nodeAddr string
var serverAddr string
var floodUpdate int = 0

func main() {
	neighboursTable = make(map[string]neighbour)
	fileStreamsAvailable = make(map[string]chan []byte)
	streamViewers = make(map[string]int)

	flag.StringVar(&bootstrapperAddr, "bs", "", "sets the ip address of the bootstrapper node")
	flag.BoolVar(&rpFlag, "rp", false, "sets the node as the overlay's Rendezvous Point")
	flag.StringVar(&serverAddr, "s", "", "sets the server wich the RP has connection to")
	flag.StringVar(&nodeAddr, "ip", "", "sets the node ip")

	var debugFlag bool
	flag.BoolVar(&debugFlag, "d", false, "turn on logs")

	flag.Parse()

	if bootstrapperAddr == "" {
		// corre bootstrapper em segundo plano
		go bootstrapper.Run(nodeAddr)
		bootstrapperAddr = nodeAddr
	}

	if debugFlag {
		log.SetOutput(io.Discard)
	}

	// faz pedido ao bootstrapper e guarda vizinhos
	setup()

	// à escuta de pedidos de outros nodos
	listener, erro := net.Listen("tcp", nodeAddr+":8081")
	if erro != nil {
		log.Println("Erro:", erro)
		return
	}
	defer listener.Close()

	log.Println("Node à escuta na porta 8081")
	for {
		client, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro:", err)
			continue
		}

		go handleRequest(client)
	}

	select {}

}

// Faz pedido ao Bootstrapper para ter os seus respetivos vizinhos
func setup() {
	if rpFlag && serverAddr != "" {
		neighboursTable[serverAddr] = neighbour{
			flag:    4,
			latency: 0,
			loss:    0,
		}
	}

	conn, erro := net.Dial("tcp", bootstrapperAddr+":8080")
	if erro != nil {
		// problema na comunicação por localhost no xubuntu core
		ducktape()
		return
	}
	defer conn.Close()

	data := []byte(nodeAddr)
	_, err := conn.Write(data)
	if err != nil {
		log.Println("[SETUP] Erro:", err)
		return
	}

	decoder := gob.NewDecoder(conn)

	var neighboursArray []string
	err = decoder.Decode(&neighboursArray)
	if err != nil {
		log.Println("[SETUP] Erro a receber vizinhos: ", err)
		return
	}

	addNeighbours(neighboursArray)

	log.Println("[SETUP] Recebi os vizinhos ", neighboursArray)
}

func ducktape() {
	n, _ := bootstrapper.GetNeighbours2(nodeAddr, bootstrapper.LoadTree2())
	addNeighbours(n)
}

func debug() {
	for {
		log.Println("[DEBUG] NEIGHBOUR STATUS")
		for ip, neighbour := range neighboursTable {
			log.Printf("IP: %s\n", ip)
			log.Printf("Type: %d - ", neighbour.flag)
			switch neighbour.flag {
			case 0:
				log.Printf("BECO\n")
			case 1:
				log.Printf("TEM CONEXÃO\n")
			case 2:
				log.Printf("MELHOR MÉTRICA\n")
			case 3:
				log.Printf("RENDEZVOUS POINT\n")
			case 4:
				log.Printf("SERVIDOR\n")
			}
			log.Printf("Latency: %d\n", neighbour.latency)
			log.Printf("Loss: %d\n", neighbour.loss)
			log.Println("---------------------------")
		}
		log.Println()
		log.Printf("[DEBUG] TOTAL ACTIVE STREAMS: %d\n", activeStreams)
		time.Sleep(10 * time.Second)
	}
}

func handleRequest(client net.Conn) {
	// lê pacote
	decoder := gob.NewDecoder(client)

	var receivedData packet
	err := decoder.Decode(&receivedData)
	if err != nil {
		log.Println("Erro no decode da mensagem: ", err)
		return
	}

	// switch case pelo tipo (0-flood, 1-request, 2-response, 3-control)
	switch receivedData.ReqType {
	case 0:
		//flood protocol
		log.Println("Flood Packet de ", receivedData.Payload.Sender)
		if rpFlag {
			floodPivotPoint(receivedData.Payload)
		} else {
			floodRetransmit(receivedData.Payload)
		}
	case 1:
		log.Printf("Stream Request de %v por %v", receivedData.Description, receivedData.Payload.Sender)
		streamRequest(client, receivedData)
		//streamFile(client, "8888", "teste")
		// abre multicast e faz stream
	case 2:
		// recebe resposta a pedido
		log.Println("Response Packet de ", receivedData.Payload.Sender)
	case 3:
		// control packets
		log.Println("Control Packet received")
	}
}

func streamRequest(client net.Conn, receivedPkt packet) {
	filePath := receivedPkt.Description
	dataChannel, ok := fileStreamsAvailable[filePath]
	if !ok {
		// se stream não existe faz request a vizinho
		log.Println("Não tenho stream de ", filePath)

		// abre um canal para redistribuir futuros pedidos
		dataChannel := make(chan []byte, 1024)
		streamViewers[filePath] = 1
		fileStreamsAvailable[filePath] = dataChannel
		requestStream(client, filePath, receivedPkt.Payload.Sender, dataChannel)
		syncStream(dataChannel)
	} else {
		log.Println("Stream já existe, redirecionando cliente")
		streamViewers[filePath] += 1
		//envia porta para onde será enviada a stream
		clientAddr := receivedPkt.Payload.Sender
		activeStreams++
		portCalc := 8000 + activeStreams
		port := strconv.Itoa(portCalc)
		encoder := gob.NewEncoder(client)
		startSignal := packet{
			ReqType:     2,
			Description: port,
			Payload: payload{
				Sender: nodeAddr,
			},
		}

		err := encoder.Encode(startSignal)
		if err != nil {
			log.Println("Erro a enviar porta:", err)
			return
		}
		log.Println("Enviei porta destino a ", clientAddr)

		// espera que o cliente avise que está pronto
		decoder := gob.NewDecoder(client)
		var ready packet
		err = decoder.Decode(&ready)

		distributeData(client, encoder, receivedPkt.Payload.Sender, port, dataChannel)
		streamViewers[filePath] -= 1
		log.Printf("Cliente %s desconectou-se da stream %s", receivedPkt.Payload.Sender, filePath)
		if streamViewers[filePath] == 0 {
			log.Printf("Stream %s sem viewers, fechando transmissão...", filePath)
			close(dataChannel)
			delete(fileStreamsAvailable, filePath)
		}

	}

}

func syncStream(dataChannel <-chan []byte) {
	for range dataChannel {
		// Discard received data
	}
}

func requestStream(clientConn net.Conn, filePath string, clientAddr string, dataChannel chan []byte) {

	// Conta mais uma stream ativa e envia ao cliente a porta onde se poderá conectar
	activeStreams++
	portCalc := 8000 + activeStreams
	portClient := strconv.Itoa(portCalc)
	encoderClient := gob.NewEncoder(clientConn)
	startSignal := packet{
		ReqType:     2,
		Description: portClient,
		Payload: payload{
			Sender: nodeAddr,
		},
	}
	// Envia a porta
	err := encoderClient.Encode(startSignal)
	if err != nil {
		log.Println("Erro a enviar porta de destino: ", err)
		return
	}
	log.Println("Enviei porta destino a ", clientAddr)

	request := packet{
		ReqType:     1,
		Description: filePath,
		Payload: payload{
			Sender: nodeAddr,
		},
	}
	// envia aos vizinho preferido
	neighbourIP, err := getFavNeighbour()
	if err != nil {
		log.Println("Erro, vizinhos sem conexão: ", err)
		flood()
		for {
			neighbourIP, err = getFavNeighbour()
			if err == nil {
				break
			} else {
				flood()
				time.Sleep(200 * time.Millisecond)
			}
		}
	}

	// connect TCP (ip)
	sourceConn, erro := net.Dial("tcp", neighbourIP+":8081")
	if erro != nil {
		log.Println("Erro a estabelecer conexão com a source:", erro)
		return
	}
	defer sourceConn.Close()

	encoder := gob.NewEncoder(sourceConn)

	// Manda pedido de ficheiro
	err = encoder.Encode(request)
	if err != nil {
		log.Println("Erro a pedir stream a source:", err)
		return
	}
	log.Println("Enviei pedido a ", neighbourIP)

	// Recebe porta onde será transmitido
	var srcPortInfoPacket packet
	decoder := gob.NewDecoder(sourceConn)
	err = decoder.Decode(&srcPortInfoPacket)
	if err != nil {
		log.Println("Erro a obter porta de stream da source: ", err)
		return
	}
	srcPort := srcPortInfoPacket.Description

	println(srcPortInfoPacket.Description)

	if srcPortInfoPacket.Description == "404" {
		activeStreams--
		// avisa cliente/nodo atrás
		encoder := gob.NewEncoder(clientConn)
		noFileSignal := packet{
			ReqType:     2,
			Description: "404",
			Payload: payload{
				Sender: nodeAddr,
			},
		}

		err = encoder.Encode(noFileSignal)
		if err != nil {
			log.Println("Erro ao avisar cliente:", err)
			return
		}
		log.Println("FICHEIRO NÃO EXISTE: Enviei aviso a ", clientAddr)

		// fecha o canal criado para esta stream
		close(dataChannel)
		delete(fileStreamsAvailable, filePath)
	} else {
		// pronto para receber
		// abre porta de leitura do servidor
		// Resolve UDP address Server
		// envia confirmação de que ficheiro será transmitido
		confirmation := packet{
			ReqType:     2,
			Description: "200",
			Payload: payload{
				Sender: nodeAddr,
			},
		}

		err := encoderClient.Encode(confirmation)
		if err != nil {
			fmt.Println("Error encoding and sending data:", err)
			return
		}
		log.Println("Enviei confirmação da stream a ", clientAddr)

		srcAddrStr := nodeAddr + ":" + srcPort

		srcAddr, err := net.ResolveUDPAddr("udp", srcAddrStr)
		if err != nil {
			log.Println(err)
		}

		// Listen for UDP connections server
		src, err := net.ListenUDP("udp", srcAddr)
		if err != nil {
			log.Println(err)
		}
		defer src.Close()

		redirAddr, err := net.ResolveUDPAddr("udp", clientAddr+":"+portClient)
		if err != nil {
			log.Println(err)
		}

		// Dial UDP for redirection client
		redirConn, err := net.DialUDP("udp", nil, redirAddr)
		if err != nil {
			log.Println(err)
		}
		defer redirConn.Close()

		// Buffer to store incoming data
		buf := make([]byte, 2048)

		// Continuously read data from the connection and redirect it
		bufferCh := make([]byte, 2048)
		stopSig := false
		for {

			n, _, err := src.ReadFromUDP(buf)
			if err != nil {
				log.Println(err)
			}

			copy(bufferCh[:n], buf[:n])

			if !stopSig {
				_, err = redirConn.Write(buf[:n])
				if err != nil {
					log.Println(err)
					stopSig = true
					streamViewers[filePath] -= 1
					if streamViewers[filePath] == 0 {
						log.Printf("Stream %s sem viewers, fechando transmissão...", filePath)
						close(dataChannel)
						delete(fileStreamsAvailable, filePath)
						return
					}
				}
			}

			// para manter o channel ativo
			select {
			case dataChannel <- bufferCh[:n]:
				continue
			default:
				continue
			}
		}
	}
}

func distributeData(clientConn net.Conn, encoder *gob.Encoder, clientAddr string, port string, dataChannel <-chan []byte) {

	confirmation := packet{
		ReqType:     2,
		Description: "200",
		Payload: payload{
			Sender: nodeAddr,
		},
	}

	//abre porta de escrita para o cliente
	redirAddr, err := net.ResolveUDPAddr("udp", clientAddr+":"+port)
	if err != nil {
		log.Println(err)
		return
	}
	// Dial UDP for redirection client
	redirConn, err := net.DialUDP("udp", nil, redirAddr)
	if err != nil {
		log.Println(err)
		return
	}
	defer redirConn.Close()

	// Envia a porta onde irá fazer stream
	err = encoder.Encode(confirmation)
	if err != nil {
		log.Println("Erro a enviar porta de destino:", err)
		return
	}

	time.Sleep(5 * time.Second)

	for {
		// Receive data from the channel
		data := <-dataChannel
		//time.Sleep(1 * time.Second)
		// Redirect data to client

		_, err = redirConn.Write(data)
		if err != nil {
			log.Println(err)
			//reduz número de ouvintes nesta stream
			return
		}

		//println("Sent to client ", clientAddr+":"+port)
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
		if n.flag == 4 {
			return ip, nil
		}
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
	log.Println("[flood debug] Tempo total:", receivedPkt.TotalTime)
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
	if flagVal == 3 {
		neighboursTable[sender] = n
		log.Println("[flood debug] Guardei o RP ", sender)
	} else if changeNeighbourMetric(n, sender) {
		neighboursTable[sender] = n
		log.Println("[flood debug] Guardei o vizinho ", sender)
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

	retransmitPkt := packet{
		ReqType: 0,
		Payload: response,
	}

	neighbourConn, erro := net.Dial("tcp", neighbourIp+":8081")
	if erro != nil {
		log.Println(erro)
		return
	}
	defer neighbourConn.Close()

	encoder := gob.NewEncoder(neighbourConn)

	// envia pacote de retrasmissão
	err := encoder.Encode(retransmitPkt)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("[flood debug] Enviei flood packet a ", neighbourIp)
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

	neighbourConn, err := net.Dial("tcp", ip+":8081")
	if err != nil {
		log.Println(err)
		return
	}
	defer neighbourConn.Close()

	encoder := gob.NewEncoder(neighbourConn)

	// Encode and send the array through the connection
	err = encoder.Encode(retransmitPkt)
	if err != nil {
		log.Println(err)
	}
	log.Println("[flood debug] Respondi ao flood de ", ip)
}

func sendToNeighbours(ipNeighbour string, pkt packet) {
	//envia struct packet com payload e reqtype a 0
	//for com numero de vizinhos em que faço sendtoneighbour em loop

	for n := range neighboursTable {
		if n != ipNeighbour {
			timeoutDuration := 1 * time.Second
			neighbourConn, erro := net.DialTimeout("tcp", n+":8081", timeoutDuration)
			if erro != nil {
				log.Println(erro)
				return
			}
			defer neighbourConn.Close()

			encoder := gob.NewEncoder(neighbourConn)

			// Encode and send the array through the connection
			err := encoder.Encode(pkt)
			if err != nil {
				fmt.Println("[flood debug] Erro a enviar pacote:", err)
				return
			}
			log.Println("[flood debug] Enviei flood packet a ", n)
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
