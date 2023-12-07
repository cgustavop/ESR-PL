package main

import (
	"encoding/gob"
	"errors"
	"esr/node/bootstrapper"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/go-ping/ping"
)

var ErrNoFavNeighbours = errors.New("No favourite neighbour found")

type neighbour struct {
	flag    int
	latency float64
	loss    float64
}

// pacote
type packet struct {
	ReqType     int
	Description string
	Payload     payload
}
type stream struct {
	channel chan []byte
	client  string
}

type payload struct {
	Sender        string
	HourIn        time.Time
	AllNodes      []string
	BacktraceSize int
	TotalTime     float64 //ms
}

// Maps the neighbour's IP to it's related info (type flag, latency, loss)
var neighboursTable map[string]neighbour
var serverTable map[string][]string

// Maps "filePath" to the multicast group Port where the file is being streamed
var fileStreamsAvailable map[string][]stream
var streamViewers map[string]int

// hard-coded
var bootstrapperAddr string
var activeStreams int = 0
var rpFlag bool
var nodeAddr string
var serverAddr string
var floodUpdate int = 0
var format string = ".mjpeg"
var bsFile string

func main() {
	neighboursTable = make(map[string]neighbour)
	serverTable = make(map[string][]string)
	fileStreamsAvailable = make(map[string][]stream)
	streamViewers = make(map[string]int)

	flag.StringVar(&bootstrapperAddr, "bs", "", "sets the ip address of the bootstrapper node")
	flag.BoolVar(&rpFlag, "rp", false, "sets the node as the overlay's Rendezvous Point")
	flag.StringVar(&serverAddr, "s", "", "sets the server wich the RP has connection to")
	flag.StringVar(&nodeAddr, "ip", "", "sets the node ip")
	flag.StringVar(&bsFile, "tree", "", "bootstrapper's nodes list")

	var debugFlag bool
	flag.BoolVar(&debugFlag, "d", true, "switch debug (default true)")

	flag.Parse()

	if bootstrapperAddr == "" {
		// corre bootstrapper em segundo plano
		go bootstrapper.Run(nodeAddr, bsFile)
		bootstrapperAddr = nodeAddr
	}

	if !debugFlag {
		log.SetOutput(io.Discard)
	} else {
		go debug()
	}

	// faz pedido ao bootstrapper e guarda vizinhos
	setup()

	if rpFlag {
		go monitor()
	}

	//go control()
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
	conn, erro := net.Dial("tcp", bootstrapperAddr+":8080")
	if erro != nil {
		// problema na comunicação por localhost no xubuntu core
		nodes, servers := bootstrapper.LocalBS(nodeAddr, bsFile)
		nodeinfo := bootstrapper.NodeInfo{
			Nodes:   nodes,
			Servers: servers,
		}
		addNeighbours(nodeinfo)

		log.Printf("[SETUP] Recebi os vizinhos %v\n", nodeinfo)
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

	var neighboursInfo bootstrapper.NodeInfo
	err = decoder.Decode(&neighboursInfo)
	if err != nil {
		log.Println("[SETUP] Erro a receber vizinhos: ", err)
		return
	}

	addNeighbours(neighboursInfo)

	log.Printf("[SETUP] Recebi os vizinhos %v\n", neighboursInfo)
}

func debug() {
	for {
		fmt.Println("[DEBUG] NEIGHBOUR STATUS")
		for ip, neighbour := range neighboursTable {
			fmt.Printf("IP: %s\n", ip)
			fmt.Printf("Type: %d - ", neighbour.flag)
			switch neighbour.flag {
			case 0:
				fmt.Printf("BECO\n")
			case 1:
				fmt.Printf("TEM CONEXÃO\n")
			case 2:
				fmt.Printf("MELHOR MÉTRICA\n")
			case 3:
				fmt.Printf("RENDEZVOUS POINT\n")
			case 4:
				fmt.Printf("SERVIDOR\n")
			}
			fmt.Printf("Latency: %.2fms\n", neighbour.latency)
			fmt.Printf("Loss: %.2f%%\n", neighbour.loss)
			fmt.Println("---------------------------")
		}
		fmt.Println()
		fmt.Printf("[DEBUG] TOTAL ACTIVE STREAMS: %d\n", activeStreams)
		time.Sleep(15 * time.Second)
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
		color.Blue("Flood Packet de " + receivedData.Payload.Sender)
		if rpFlag {
			floodPivotPoint(receivedData.Payload)
		} else {
			floodRetransmit(receivedData.Payload)
		}
	case 1:
		color.Blue("Stream Request de " + receivedData.Description + " por " + receivedData.Payload.Sender)
		streamRequest(client, receivedData)
		//streamFile(client, "8888", "teste")
		// abre multicast e faz stream
	case 2:
		// recebe control packets
		log.Println("Control Packet received")
		//control()
	}
}

func control() {

	time.Sleep(30 * time.Second)
	for target, n := range neighboursTable {
		pinger, err := ping.NewPinger(target)
		if err != nil {
			log.Println("Erro a enviar ping a vizinho:", err)
			updateNeighbour(target, 0, 0, 0)
			continue
		}
		pinger.SetPrivileged(true)
		pinger.Count = 10 // pacotes a serem enviados
		pinger.Timeout = 5 * time.Second

		pinger.OnRecv = func(pkt *ping.Packet) {
			log.Printf("%d bytes from %s: icmp_seq=%d time=%v\n",
				pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
		}

		pinger.OnFinish = func(stats *ping.Statistics) {
			log.Printf("\n--- %s ping statistics ---\n", target)
			log.Printf("%d pacote enviados, %d pacotes recebidos, %.2f%% packet loss\n",
				stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
		}

		fmt.Printf("Ping a %s com %d pacotes:\n", target, pinger.Count)
		err = pinger.Run()
		if err != nil {
			log.Println("Erro a enviar ping a vizinho:", err)
			updateNeighbour(target, 0, 0, 0)
			continue
		}
		stats := pinger.Statistics()

		flag := n.flag
		rtt := stats.AvgRtt.Seconds() * 1000
		loss := float64(stats.PacketLoss)
		updateNeighbour(target, flag, rtt, loss)
	}
	updateFavNeighbour()
}

func updateNeighbour(ip string, flag int, rtt float64, loss float64) {
	update := neighbour{
		flag:    flag,
		latency: rtt,
		loss:    loss,
	}
	neighboursTable[ip] = update
}

func streamRequest(client net.Conn, receivedPkt packet) {
	filePath := receivedPkt.Description
	clientAddr := receivedPkt.Payload.Sender

	dataChannel := make(chan []byte, 2048)
	activeStreams++
	ok := len(fileStreamsAvailable[filePath]) > 0
	if ok {
		newStream := stream{
			channel: dataChannel,
			client:  clientAddr,
		}

		color.Green("Stream já existe, redirecionando cliente")
		streamViewers[filePath] += 1
		fileStreamsAvailable[filePath] = append(fileStreamsAvailable[filePath], newStream)

		clientUDP, err := handshake(client, clientAddr, "200")
		if err != nil {
			log.Println("cliente UDP not ready")
			go deleteStreamEntry(filePath, clientAddr)
			streamViewers[filePath] -= 1
			activeStreams--
			return
		}
		retransmit(clientUDP, filePath, dataChannel)
		go deleteStreamEntry(filePath, clientAddr)
		streamViewers[filePath] -= 1
		activeStreams--
	} else {
		newStream := stream{
			channel: dataChannel,
			client:  clientAddr,
		}
		// se stream não existe faz request a vizinho
		color.Green("Não tenho stream de " + filePath)
		streamViewers[filePath] += 1
		fileStreamsAvailable[filePath] = append(fileStreamsAvailable[filePath], newStream)
		statusChannel := make(chan string)
		go getStream(client, filePath, clientAddr, statusChannel)

		status, _ := <-statusChannel
		clientUDP, err := handshake(client, clientAddr, status)
		if err != nil {
			log.Println("cliente UDP not ready")
			go deleteStreamEntry(filePath, clientAddr)
			activeStreams--
			return
		}
		retransmit(clientUDP, filePath, dataChannel)
		streamViewers[filePath] -= 1
		activeStreams--
		go deleteStreamEntry(filePath, clientAddr)
	}

}

func deleteStreamEntry(key string, id string) {
	streams := fileStreamsAvailable[key]

	for i, stream := range streams {
		if stream.client == id {
			// Delete the stream from the slice
			fileStreamsAvailable[key] = append(streams[:i], streams[i+1:]...)
			for len(stream.channel) > 0 {
				_ = <-stream.channel
			}
			close(stream.channel)
		}
	}
}

func handshake(clientTCPConn net.Conn, clientAddr string, status string) (*net.UDPConn, error) {
	encoder := gob.NewEncoder(clientTCPConn)

	confirmation := packet{
		ReqType:     1,
		Description: status,
		Payload: payload{
			Sender: clientAddr,
		},
	}

	// envia confirmação a cliente
	errCon := encoder.Encode(confirmation)
	if errCon != nil {
		log.Println("Erro a enviar confirmação a cliente:", errCon)
		return nil, errCon
	}
	log.Println("Enviei confirmação a ", clientAddr)

	//abre porta de escrita para o cliente
	color.Green("Enviando ao cliente em " + clientAddr)
	redirAddr, err := net.ResolveUDPAddr("udp", clientAddr)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	// Dial UDP for redirection client
	redirConn, err := net.DialUDP("udp", nil, redirAddr)
	if err != nil {
		log.Println(err)
		redirConn.Close()
		return nil, err
	}

	return redirConn, nil
	//time.Sleep(5 * time.Second)
}

func retransmit(clientUDPConn *net.UDPConn, filePath string, dataChannel chan []byte) {
	defer clientUDPConn.Close()
	for {
		// Receive data from the channel
		data := <-dataChannel
		// Redirect data to client

		_, err := clientUDPConn.Write(data)
		if err != nil {
			color.Red("CLIENTE FECHOU CONEXÃO")
			log.Println(err)
			return
		}

		//println("Sent to client ", clientAddr+":"+port)
	}
}

func source(filePath string) string {
	var ip string
	if rpFlag {
		updateServerFileList()
		ip = chooseSource(filePath)
		return ip
	} else {
		updateFavNeighbour()
		ip, err := getFavNeighbour()
		if err == nil {
			return ip
		} else {
			flood()
			updateFavNeighbour()
			try := 3
			for {
				updateFavNeighbour()
				ip, err := getFavNeighbour()
				if err == nil {
					return ip
				} else if try == 0 {
					return ""
				} else {
					time.Sleep(500 * time.Millisecond)
					flood()
					updateFavNeighbour()
				}
				try--
			}
		}

	}

}

func getStream(clientConn net.Conn, filePath string, clientAddr string, statusChannel chan string) {
	portCalc := 8000 + activeStreams
	portLocal := strconv.Itoa(portCalc)

	request := packet{
		ReqType:     1,
		Description: filePath,
		Payload: payload{
			Sender: nodeAddr + ":" + portLocal,
		},
	}

	// pede ao vizinho preferido
	ip := source(filePath)
	if ip == "" {
		statusChannel <- "500"
		return
	}
	color.Magenta("HERE" + ip)
	// connect TCP (ip)
	sourceConn, errCon := net.Dial("tcp", ip+":8081")
	if errCon != nil {
		log.Println("Erro a estabelecer conexão com a source:", errCon)
		statusChannel <- "404"
		return
	}
	defer sourceConn.Close()

	encoder := gob.NewEncoder(sourceConn)

	// envia pedido de ficheiro
	errCon = encoder.Encode(request)
	if errCon != nil {
		log.Println("Erro a pedir stream a source:", errCon)
		statusChannel <- "500"
		return
	}
	log.Println("Enviei pedido a ", ip)

	// recebe confirmação
	var confirmation packet
	decoder := gob.NewDecoder(sourceConn)
	errCon = decoder.Decode(&confirmation)
	if errCon != nil {
		log.Println("Erro a obter porta de stream da source: ", errCon)
		statusChannel <- "500"
		return
	}
	//srcPort := srcPortInfoPacket.Description
	if confirmation.Description == "500" {
		statusChannel <- "500"
		return
	} else if confirmation.Description == "404" {
		statusChannel <- "404"
		return
	} else {
		statusChannel <- "200"
		close(statusChannel)
		srcAddrStr := nodeAddr + ":" + portLocal

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

		// Buffer to store incoming data
		buf := make([]byte, 2048)
		// Continuously read data from the connection and redirect it
		bufferCh := make([]byte, 2048)
		for {
			n, _, err := src.ReadFromUDP(buf)
			if err != nil {
				log.Println(err)
			}
			copy(bufferCh[:n], buf[:n])

			for _, stream := range fileStreamsAvailable[filePath] {
				stream.channel <- bufferCh[:n]
			}

			if len(fileStreamsAvailable[filePath]) == 0 {
				color.Cyan("No one's connected. Closing " + filePath + " stream...")
				return
			}
			// cleaning the buffer
			buf = []byte{}
			buf = make([]byte, 2048)
		}
	}
}

// devolve o melhor vizinho de uma lista de ips dadas as suas métricas
func chooseNeighbour(list []string) string {
	// atribuir pesos a perdas e a latencia
	pesoLatency := 0.6
	pesoPacketLoss := 0.4

	bestScore := math.Inf(1)
	var bestNode string

	for s, n := range neighboursTable {
		if stringInArray(s, list) {
			score := (pesoLatency * n.latency) + (pesoPacketLoss * n.loss)
			// verificar score
			if score < bestScore {
				bestScore = score
				bestNode = s
			}
		}
	}

	return bestNode
}

func updateFavNeighbour() {
	pesoLatency := 0.8
	pesoPacketLoss := 0.2

	bestScore := 100000000000.0
	var bestNode string

	if len(neighboursTable) > 0 {
		return
	}

	removeFav()

	for s, n := range neighboursTable {
		if n.flag == 2 || n.flag == 1 {
			score := (pesoLatency * n.latency) + (pesoPacketLoss * n.loss / 100)
			// verificar score
			if score < bestScore {
				bestScore = score
				bestNode = s
			}
		}
	}

	setNewFav(bestNode)
}

func removeFav() {
	for s, n := range neighboursTable {
		if n.flag == 2 {
			update := neighbour{
				flag:    1,
				latency: n.latency,
				loss:    n.loss,
			}
			neighboursTable[s] = update
		}
	}
}

func setNewFav(ip string) {
	n := neighboursTable[ip]
	update := neighbour{
		flag:    2,
		latency: n.latency,
		loss:    n.loss,
	}
	neighboursTable[ip] = update
}

// devolve melhor servidor para um ficheiro
func chooseSource(filepath string) string {
	sources := []string{}
	fullFP := filepath + format
	for s, files := range serverTable {
		if stringInArray(fullFP, files) {
			sources = append(sources, s)
		}
	}
	src := chooseNeighbour(sources)
	return src
}

func stringInArray(str string, list []string) bool {
	for _, e := range list {
		if e == str {
			return true
		}
	}
	return false
}

func updateServerFileList() {
	var wg sync.WaitGroup
	for s, _ := range serverTable {
		wg.Add(1)
		go getServerFileList(s, &wg)
	}
	wg.Wait()
}

func getServerFileList(serverAddr string, wg *sync.WaitGroup) {
	defer wg.Done()
	// envia pedido ao servidor tipo 3
	// recebe array
	request := packet{
		ReqType:     2,
		Description: "streams available",
		Payload: payload{
			Sender: nodeAddr,
		},
	}

	sourceConn, errCon := net.DialTimeout("tcp", serverAddr+":8081", 5*time.Second)
	if errCon != nil {
		log.Println("Erro a estabelecer conexão com a source:", serverAddr, errCon)
		return
	}
	defer sourceConn.Close()
	startTime := time.Now()
	encoder := gob.NewEncoder(sourceConn)

	// Manda pedido de lista de ficheiros
	errCon = encoder.Encode(request)
	if errCon != nil {
		log.Println("Erro a pedir stream a source:", errCon)
		return
	}
	log.Println("Pedi lista de ficheiros ao ", serverAddr)

	availableFiles := []string{}
	decoder := gob.NewDecoder(sourceConn)
	decoder.Decode(&availableFiles)
	rtt := time.Since(startTime)
	fmt.Printf("%v\n", availableFiles)
	serverTable[serverAddr] = availableFiles
	update := neighbour{
		flag:    4,
		latency: float64(rtt.Milliseconds()),
	}
	if changeNeighbourMetric(update, serverAddr) {
		neighboursTable[serverAddr] = update
		log.Println("Updating server", serverAddr, "metrics")
	}
}

func addNeighbours(info bootstrapper.NodeInfo) {
	servers := info.Servers
	nodes := info.Nodes
	for _, n := range nodes {
		neighboursTable[n] = neighbour{
			flag:    0, //inicialmente sem ligação ao RP
			latency: 0.0,
			loss:    0.0,
		}
	}

	if len(servers) > 0 {
		rpFlag = true
		for _, n := range servers {
			neighboursTable[n] = neighbour{
				flag:    4, //indica que é servidor
				latency: 0.0,
				loss:    0.0,
			}
			serverTable[n] = []string{}
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
		flagVal := 1
		if len(receivedPkt.AllNodes) == receivedPkt.BacktraceSize+2 {
			flagVal = 3
		}
		updateNeighbour(ipNeighbour, flagVal, receivedPkt.TotalTime, 0)
		updateFavNeighbour()
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
		latency: float64(pkt.TotalTime),
		loss:    0.0,
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
	timeout := 5 * time.Second
	neighbourConn, erro := net.DialTimeout("tcp", neighbourIp+":8081", timeout)
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
	duration := currentTime.Sub(receivedPkt.HourIn).Milliseconds()

	response := payload{
		Sender:        nodeAddr,
		HourIn:        receivedPkt.HourIn,
		AllNodes:      a,
		BacktraceSize: receivedPkt.BacktraceSize,
		TotalTime:     float64(duration),
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
			timeoutDuration := 5 * time.Second
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

func monitor() {
	time.Sleep(20 * time.Second)
	log.Println("Monitoring servers...")
	updateServerFileList()
}
