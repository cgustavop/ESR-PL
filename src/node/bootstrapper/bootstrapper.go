package bootstrapper

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

var tree map[string][]string

func Run() {
	loadTree()
	udpAddr, err := net.ResolveUDPAddr("udp", "localhost:8080")

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	listener, erro := net.ListenUDP("udp", udpAddr)
	if erro != nil {
		fmt.Println("Error:", erro)
		return
	}
	defer listener.Close()

	fmt.Println("Server is listening on port 8080")

	for {
		var buf [512]byte
		_, addr, err := listener.ReadFromUDP(buf[0:])
		if err != nil {
			fmt.Println(err)
			return
		}

		go handleRequest(listener, addr) // go X arranca uma nova thread que executa a função X
		// Write back the message over UPD

	}
}

func loadTree() {
	path := "tree.json"
	// carrega ficheiro json
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	tree = make(map[string][]string)

	if err := decoder.Decode(&tree); err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}

	for key, value := range tree {
		fmt.Printf("Key: %s\n", key)
		fmt.Printf("Array Values: %v\n", value)
	}
}

func getNeighbours(ip string) ([]string, error) {
	fmt.Printf("Request from %s\n", ip)

	value, found := tree[ip]
	if !found {
		return []string{}, fmt.Errorf("Key not found in the map: %s", ip)
	}
	return value, nil
}

func handleRequest(conn *net.UDPConn, client *net.UDPAddr) {
	neighboursArray, _ := getNeighbours(client.IP.String())

	arrayStr := fmt.Sprintf("%v", neighboursArray)
	data := []byte(arrayStr)

	conn.WriteToUDP(data, client)
}
