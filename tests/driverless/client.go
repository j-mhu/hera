package main

import (
	"net"
	"fmt"
	"bufio"
	"os"
)

func main() {
	conn, _ := net.Dial("tcp", "0.0.0.0:3333")
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Text to send: ")
		text, _ := reader.ReadString('\n')
		fmt.Fprintf(conn, text + "\n")
		message, _ := bufio.NewReader(conn).ReadByte()

		fmt.Println(message)

	}

}
