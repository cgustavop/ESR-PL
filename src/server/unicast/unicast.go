package unicast

import (
	"fmt"
	"net"
	"os"
)

// recebe um path
// packetization do ficheiro io.Read
// enviar pacote
// garantir que recebeu (esperar ACK)
// retransmissão se perdeu pacotes ou se deu timeout (conta retransmissões)
// enviar restantes
// end of trasmission (EOT)

// DO OUTRO LADO
// receber pacotes (apontar ordem) é EOT ou packet
// mandar ACK -> número do pacote que recebeu
// reassembly

func SendFile(conn *net.UDPConn, client *net.UDPAddr, filePath string) {
	//fmt.Printf("Request for %s from %s\n", filePath, client.IP.String())
	//data := []byte("ACK")
	//conn.WriteToUDP(data, client)
	//	io.Read()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Set buffer size
	bufSize := 1024
	buffer := make([]byte, bufSize)

	// Read and send the file in chunks
	for {
		// Read a chunk of the file
		n, err := file.Read(buffer)
		if err != nil {
			break
		}

		chunk := buffer[:n]

		if n < bufSize {
			sendChunk(conn, client, chunk)
			sendEOF(conn, client)
			checkEOF(conn, client)
			fmt.Println("File sent successfully.")
			return
		}
		fmt.Println("huh.")
		sendChunk(conn, client, chunk)
		checkACK(conn, client)

	}

}

// Send the chunk over UDP
func sendChunk(conn *net.UDPConn, client *net.UDPAddr, chunk []byte) {
	_, err := conn.WriteToUDP(chunk, client)
	if err != nil {
		fmt.Println("Error sending chunk:", err)
		return
	}
}

func checkACK(conn *net.UDPConn, clien *net.UDPAddr) {
	var buf [1024]byte
	msg, _, err := conn.ReadFromUDP(buf[0:])
	ackMsg := string(buf[:msg])
	if err != nil || ackMsg != "ACK" {
		fmt.Println(err)
		return
	}
}

// EOF signal
func sendEOF(conn *net.UDPConn, client *net.UDPAddr) {
	eof := []byte("EOF")
	conn.WriteToUDP(eof, client)
}

func checkEOF(conn *net.UDPConn, clien *net.UDPAddr) {
	var buf [1024]byte
	msg, _, err := conn.ReadFromUDP(buf[0:])
	eofMsg := string(buf[:msg])
	if err != nil || eofMsg != "EOF" {
		fmt.Println(err)
		return
	}
}
