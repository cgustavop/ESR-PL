package bootstrapper

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
)

type NodeInfo struct {
	Nodes   []string `json:"nodes"`
	Servers []string `json:"servers,omitempty"`
}

var tree map[string][]string
var list map[string]NodeInfo

func Run(ip string, treeFile string) {
	path := "node/" + treeFile
	setup(path)

	addr := ip + ":8080"
	listener, erro := net.Listen("tcp", addr)
	if erro != nil {
		log.Println("[Bootstrapper] Erro:", erro)
		return
	}
	defer listener.Close()

	log.Println("[Bootstrapper] À escuta em ", addr)

	for {
		client, err := listener.Accept()
		if err != nil {
			log.Println("[Bootstrapper] Erro:", err)
			continue
		}
		go handleRequest(client)
	}

}

func getNodesAndServersForIP(ipAddress string) ([]string, []string) {
	info, found := list[ipAddress]
	if !found {
		log.Println("[Bootstrapper] Vizinhos de", ipAddress, "não encontrados")
		return []string{}, []string{}
	}

	return info.Nodes, info.Servers
}

func setup(path string) {
	// carrega ficheiro json
	file, err := os.Open(path)
	if err != nil {
		log.Println("[Bootstrapper] Erro a abrir JSON:", err)
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&list); err != nil {
		log.Println("[Bootstrapper] Erro a ler JSON:", err)
		return
	}
}

// return nodes, servers arrays
func connections(ipAddress string) ([]string, []string) {
	// Get nodes and servers for a specific IP address
	nodes, servers := getNodesAndServersForIP(ipAddress)

	// Print the nodes and servers for the given IP address
	fmt.Println("[Bootstrapper] Nodes for", ipAddress, ":", nodes)
	fmt.Println("[Bootstrapper] Servers for", ipAddress, ":", servers)

	return nodes, servers
}

func handleRequest(conn net.Conn) {
	request := make([]byte, 4096)
	msg, err := conn.Read(request)
	if err != nil {
		fmt.Println("[Bootstrapper]", err)
		return
	}

	var ip string = string(request[:msg])

	nodes, servers := connections(ip)

	nodeNeighbours := NodeInfo{
		Nodes:   nodes,
		Servers: servers,
	}

	encoder := gob.NewEncoder(conn)

	// Encode and send the array through the connection
	err = encoder.Encode(nodeNeighbours)
	if err != nil {
		log.Println("[Bootstrapper]", err)
		return
	}
	log.Println("[Bootstrapper] Vizinhos enviados ", nodes, "/ Servers enviados ", servers)

}

// returns nodes, servers arrays
func LocalBS(ip string, treeFile string) ([]string, []string) {
	path := "node/" + treeFile
	setup(path)
	nodes, servers := connections(ip)
	return nodes, servers
}
