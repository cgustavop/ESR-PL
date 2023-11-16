package unicast

import (
	"fmt"
	"net"
	"os"
)

func SendFile(conn net.Conn, filePath string) (int, *net.UDPConn) {

	// connect to client using UDP
	addr, err := net.ResolveUDPAddr("udp", conn.RemoteAddr().String())
	addr.Port = 8081
	listener, erro := net.DialUDP("udp", nil, addr)
	if erro != nil {
		fmt.Println("Error:", erro)
		return 0, nil
	}
	fmt.Println("Connected at ", addr)
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return 0, nil
	}

	// Set buffer size
	bufSize := 1024
	buffer := make([]byte, bufSize)
	chunkCount := 0

	// Read and send the file in chunks
	for {
		// Read a chunk of the file
		n, err := file.Read(buffer)
		if err != nil {
			break
		}

		chunk := buffer[:n]

		_, err = listener.Write(chunk)
		if err != nil {
			fmt.Println("Error sending chunk:", err)
			return 0, nil
		}
		chunkCount++
	}

	file.Close()

	return chunkCount, listener
}
