package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

func HandleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		// Read the message length first (4 bytes for an int32)
		var length int32
		err := binary.Read(conn, binary.LittleEndian, &length)
		if err == io.EOF {
			fmt.Println("Connection closed by client")
			return
		}
		if err != nil {
			fmt.Println("Error reading message length:", err)
			return
		}

		// Read the actual message based on the length
		buf := make([]byte, length)
		_, err = io.ReadFull(conn, buf)
		if err != nil {
			fmt.Println("Error reading message:", err)
			return
		}

		// Decode the base message to check its type
		var base BaseMessage
		err = json.Unmarshal(buf, &base)
		if err != nil {
			fmt.Println("Error decoding base message:", err)
			return
		}

	}
}
