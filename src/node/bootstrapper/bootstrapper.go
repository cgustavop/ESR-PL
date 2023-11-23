package bootstrapper

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

var tree map[string][]string

func Run() {
	loadTree()

	listener, erro := net.Listen("tcp", "localhost:8080")
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
	/*
		for key, value := range tree {
			fmt.Printf("Key: %s\n", key)
			fmt.Printf("Array Values: %v\n", value)
		}
	*/
}

func getNeighbours(ip string) ([]string, error) {
	fmt.Printf("Request from %s\n", ip)

	value, found := tree[ip]
	if !found {
		return []string{}, fmt.Errorf("Vizinho não se encontra no ficheiro JSON: %s", ip)
	}
	return value, nil
}

func handleRequest(conn net.Conn) {
	request := make([]byte, 4096)
	msg, err := conn.Read(request)
	if err != nil {
		fmt.Println("Erro a ler mensagem do cliente:", err)
		return
	}

	r := string(request[:msg])

	if r != "RESOLVE" {
		fmt.Println("Pedido inválido")
		return
	}

	addr, _ := net.ResolveTCPAddr("tcp", conn.RemoteAddr().String())
	ip := addr.IP.String()
	neighboursArray, _ := getNeighbours(ip)
	println(ip)

	encoder := gob.NewEncoder(conn)

	// Encode and send the array through the connection
	err = encoder.Encode(neighboursArray)
	if err != nil {
		fmt.Println("Error encoding and sending data:", err)
		return
	}
	fmt.Println("Vizinhos enviados ", neighboursArray)

}
