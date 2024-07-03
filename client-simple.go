package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: client <server address> <client key>")
		return
	}

	serverAddress := os.Args[1]
	clientKey := os.Args[2]

	localAddr := &net.UDPAddr{
		Port: 0,
		IP:   net.ParseIP("0.0.0.0"),
	}
	serverAddr, err := net.ResolveUDPAddr("udp4", serverAddress+":8085")
	if err != nil {
		panic(err)
	}

	// Create a UDP connection that can send/receive from any address
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Send key to server
	_, err = conn.WriteToUDP([]byte(clientKey), serverAddr)
	if err != nil {
		panic(err)
	}

	// Wait for response from server
	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		panic(err)
	}

	otherClientAddress := string(buffer[:n])
	fmt.Println("Received other client address:", otherClientAddress)

	// Convert the received string address to a UDP address
	udpAddr, err := net.ResolveUDPAddr("udp4", otherClientAddress)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			// Sending "Ping" to the other client every 2 seconds
			_, err := conn.WriteToUDP([]byte("Ping from "+clientKey), udpAddr)
			if err != nil {
				fmt.Println("Error sending ping:", err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	for {
		// Listen for messages from the other client
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			continue
		}
		fmt.Println("Received:", string(buffer[:n]))
	}
}

