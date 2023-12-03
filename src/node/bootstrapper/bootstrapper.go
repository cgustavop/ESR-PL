package bootstrapper

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

var tree map[string][]string

func Run(ip string) {
	loadTree()
	addr := ip+":8080"
	listener, erro := net.Listen("tcp", addr)
	if erro != nil {
		fmt.Println("[bs] Error:", erro)
		return
	}
	defer listener.Close()
	
	fmt.Println("[bs] Bootstrapper is listening on ", addr)

	for {
		client, err := listener.Accept()
		if err != nil {
			fmt.Println("[bs] Error:", err)
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
		fmt.Println("[bs] Error opening file:", err)
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	tree = make(map[string][]string)

	if err := decoder.Decode(&tree); err != nil {
		fmt.Println("[bs] Error decoding JSON:", err)
		return
	}
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
		fmt.Println("[bs] Erro a ler mensagem do cliente:", err)
		return
	}
	println(string(request[:msg]))
	
	var ip string = string(request[:msg])

	neighboursArray, _ := getNeighbours(ip)
	println(ip)

	encoder := gob.NewEncoder(conn)

	// Encode and send the array through the connection
	err = encoder.Encode(neighboursArray)
	if err != nil {
		fmt.Println("[bs] Error encoding and sending data:", err)
		return
	}
	fmt.Println("[bs] Vizinhos enviados ", neighboursArray)

}

func LoadTree2() map[string][]string {
	path := "tree.json"
	// carrega ficheiro json
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("[bs] Error opening file:", err)
		return make(map[string][]string)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	tree := make(map[string][]string)

	if err := decoder.Decode(&tree); err != nil {
		fmt.Println("[bs] Error decoding JSON:", err)
		return make(map[string][]string)
	}

	return tree
}

func GetNeighbours2(ip string, tree map[string][]string ) ([]string, error) {
	value, found := tree[ip]
	if !found {
		return []string{}, fmt.Errorf("Vizinho não se encontra no ficheiro JSON: %s", ip)
	}
	return value, nil
}