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

		// Send the chunk over UDP
		_, err = conn.Write(buffer[:n])
		if err != nil {
			fmt.Println("Error sending data:", err)
			return
		}
	}

	fmt.Println("File sent successfully.")

}
