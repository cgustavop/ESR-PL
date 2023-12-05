package bootstrapper

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
)

var tree map[string][]string

func Run(ip string) {
	loadTree()
	addr := ip + ":8080"
	listener, erro := net.Listen("tcp", addr)
	if erro != nil {
		log.Println("[Bootstrapper] Erro:", erro)
		return
	}
	defer listener.Close()

	log.Println("[Bootstrapper] À escuta na porta ", addr)

	for {
		client, err := listener.Accept()
		if err != nil {
			log.Println("[Bootstrapper] Erro:", err)
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
		log.Println("[Bootstrapper] Erro a abrir JSON:", err)
		return
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	tree = make(map[string][]string)

	if err := decoder.Decode(&tree); err != nil {
		fmt.Println("[Bootstrapper] Erro a ler JSON:", err)
		return
	}
}

func getNeighbours(ip string) ([]string, error) {
	log.Printf("[Bootstrapper] Novo pedido de %s\n", ip)

	value, found := tree[ip]
	if !found {
		return []string{}, fmt.Errorf("[Bootstrapper] Vizinho não se encontra listado: %s", ip)
	}
	return value, nil
}

func handleRequest(conn net.Conn) {
	request := make([]byte, 4096)
	msg, err := conn.Read(request)
	if err != nil {
		fmt.Println("[Bootstrapper]", err)
		return
	}

	var ip string = string(request[:msg])

	neighboursArray, _ := getNeighbours(ip)

	encoder := gob.NewEncoder(conn)

	// Encode and send the array through the connection
	err = encoder.Encode(neighboursArray)
	if err != nil {
		log.Println("[Bootstrapper]", err)
		return
	}
	log.Println("[Bootstrapper] Vizinhos enviados ", neighboursArray)

}

func LoadTree2() map[string][]string {
	path := "tree.json"
	// carrega ficheiro json
	file, err := os.Open(path)
	if err != nil {
		log.Println("[Bootstrapper] Erro a abrir JSON:", err)
		return make(map[string][]string)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	tree := make(map[string][]string)

	if err := decoder.Decode(&tree); err != nil {
		log.Println("[Bootstrapper] Erro a ler JSON:", err)
		return make(map[string][]string)
	}

	return tree
}

func GetNeighbours2(ip string, tree map[string][]string) ([]string, error) {
	value, found := tree[ip]
	if !found {
		return []string{}, fmt.Errorf("[Bootstrapper] Vizinho não se encontra listado: %s", ip)
	}
	return value, nil
}
