package main

import (
        "fmt"
        "net"
        "sync"
)

var clientMap = make(map[string]*net.UDPAddr)
var mutex = &sync.Mutex{}

func main() {
        address := ":8085"
        udpAddress, err := net.ResolveUDPAddr("udp4", address)
        if err != nil {
                panic(err)
        }

        conn, err := net.ListenUDP("udp", udpAddress)
        if err != nil {
                panic(err)
        }
        defer conn.Close()

        buffer := make([]byte, 1024)
        for {
                n, addr, err := conn.ReadFromUDP(buffer)
                if err != nil {
                        fmt.Println("Error:", err)
                        continue
                }

                clientKey := string(buffer[:n])
                fmt.Printf("Client connected: %s (IP: %s, Port: %d)\n", clientKey, addr.IP.String(), addr.Port)

                mutex.Lock()
                if existingClient, ok := clientMap[clientKey]; ok {
                        // If the client key exists, send back the other client's address
                        conn.WriteToUDP([]byte(existingClient.String()), addr)
                        conn.WriteToUDP([]byte(addr.String()), existingClient)
                        delete(clientMap, clientKey)
                } else {
                        clientMap[clientKey] = addr
                }
                mutex.Unlock()
        }
}

