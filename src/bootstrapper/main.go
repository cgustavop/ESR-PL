package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

var tree map[string][]string

func main() {
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

		go handleRequest(client) // go arranca numa nova thread
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

func getNeighbours(key string) ([]string, error) {
	ip, _, _ := net.SplitHostPort(key)
	fmt.Printf("Request from %s\n", ip)

	value, found := tree[ip]
	if !found {
		return []string{}, fmt.Errorf("Key not found in the map: %s", key)
	}
	return value, nil
}

func handleRequest(client net.Conn) {
	var addr string = client.RemoteAddr().String()
	neighboursArray, err := getNeighbours(addr)

	arrayStr := fmt.Sprintf("%v", neighboursArray)
	data := []byte(arrayStr)

	_, err = client.Write(data)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	defer client.Close()
}
