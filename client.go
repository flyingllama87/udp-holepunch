package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

var terminateFlag int32 = 0

func main() {
	server := flag.String("server", "", "The server address.")
	clientKey := flag.String("key", "", "The client key.")
	pps := flag.Int("pps", 10, "Packets per second.")

	flag.Parse()

	if *server == "" || *clientKey == "" {
		fmt.Println("Usage: client --server <server address> --key <client key> [--pps <packets per second>]")
		return
	}

	localAddr := &net.UDPAddr{
		Port: 0,
		IP:   net.ParseIP("0.0.0.0"),
	}
	serverAddr, err := net.ResolveUDPAddr("udp4", *server+":8085")
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
	_, err = conn.WriteToUDP([]byte(*clientKey), serverAddr)
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

	// Define atomic counters and other variables
	var sentPackets int64 = 0
	var receivedPackets int64 = 0
	var receivedBytes int64 = 0
	var totalSentPackets int64 = 0
	var totalReceivedPackets int64 = 0
	var totalReceivedBytes int64 = 0
	var totalLatency int64 = 0
	var totalLatencyCount int64 = 0

	// Sync start time with the other client
	startTime := time.Now().UnixNano()
	binary.BigEndian.PutUint64(buffer[:8], uint64(startTime))
	_, err = conn.WriteToUDP(buffer[:8], udpAddr)
	if err != nil {
		panic(err)
	}
	n, _, err = conn.ReadFromUDP(buffer[:8])
	if err != nil {
		panic(err)
	}
	startTime = int64(binary.BigEndian.Uint64(buffer[:8]))

	// Handle graceful exit and display total summary
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		atomic.StoreInt32(&terminateFlag, 1)
		sendTerminationPacket(conn, udpAddr)
		displayTotalSummary(totalSentPackets, totalReceivedPackets, totalReceivedBytes, totalLatency, totalLatencyCount, startTime)
		os.Exit(0)
	}()

	// Send packets at the desired PPS rate
	go func() {
		for {
			if atomic.LoadInt32(&terminateFlag) == 1 {
				return
			}
			atomic.AddInt64(&sentPackets, 1)
			packetIndex := atomic.AddInt64(&totalSentPackets, 1)
			message := make([]byte, 1000)
			binary.BigEndian.PutUint64(message, uint64(packetIndex))
			binary.BigEndian.PutUint64(message[8:], uint64(time.Now().UnixNano()))
			copy(message[16:], []byte(fmt.Sprintf("Packet %d from %s", packetIndex, *clientKey)))

			_, err := conn.WriteToUDP(message, udpAddr)
			if err != nil {
				fmt.Println("Error sending packet:", err)
			}
			time.Sleep(time.Second / time.Duration(*pps))
		}
	}()

	go func() {
		for {
			n, addr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				fmt.Println("Error reading:", err)
				continue
			}
			if n == 9 && string(buffer[:n]) == "terminate" {
				fmt.Println("Received termination signal from the other client.")
				atomic.StoreInt32(&terminateFlag, 1)
				displayTotalSummary(totalSentPackets, totalReceivedPackets, totalReceivedBytes, totalLatency, totalLatencyCount, startTime)
				os.Exit(0)
			}
			atomic.AddInt64(&receivedPackets, 1)
			atomic.AddInt64(&receivedBytes, int64(n))
			atomic.AddInt64(&totalReceivedPackets, 1)
			atomic.AddInt64(&totalReceivedBytes, int64(n))

			packetIndex := binary.BigEndian.Uint64(buffer[:8])
			sendTime := int64(binary.BigEndian.Uint64(buffer[8:16]))
			latency := time.Now().UnixNano() - sendTime
			latencyMs := latency / 1e6 // convert to milliseconds
			atomic.AddInt64(&totalLatency, latencyMs)
			atomic.AddInt64(&totalLatencyCount, 1)
			message := buffer[16:n]
			fmt.Printf("Received packet index: %d, message: %s, latency: %d ms, sender IP: %s, sender Port: %d\n",
				packetIndex, string(bytes.Trim(message, "\x00")), latencyMs, addr.IP.String(), addr.Port)
		}
	}()

	// Every 5 seconds, log the packet loss rate and bandwidth, including latency
	go func() {
		for {
			time.Sleep(5 * time.Second)

			if atomic.LoadInt32(&terminateFlag) == 1 {
				return
			}

			// Capture and reset counters
			sent := atomic.SwapInt64(&sentPackets, 0)
			received := atomic.SwapInt64(&receivedPackets, 0)
			bytes := atomic.SwapInt64(&receivedBytes, 0)

			lossRate := float64(sent-received) / float64(sent) * 100

			// Bandwidth in bytes per second, can be converted to other units as needed.
			bandwidth := float64(bytes) / 5.0 // bytes/5 seconds = bytes/second
			bandwidthMiB := bandwidth / (1024 * 1024) // convert to MiB/sec

			// Calculate average latency
			totalLatency := atomic.LoadInt64(&totalLatency)
			totalLatencyCount := atomic.LoadInt64(&totalLatencyCount)
			averageLatency := float64(totalLatency) / float64(totalLatencyCount)

			fmt.Printf("Summary over the last 5 seconds:\n")
			fmt.Printf("Sent packets: %d\n", sent)
			fmt.Printf("Received packets: %d\n", received)
			fmt.Printf("Packet loss rate: %.2f%%\n", lossRate)
			fmt.Printf("Bandwidth throughput: %.2f MiB/sec\n", bandwidthMiB)
			fmt.Printf("Average latency: %.2f ms\n", averageLatency)
			fmt.Println("-------------------------------")
		}
	}()

	select {} // Keep the main function running
}

func sendTerminationPacket(conn *net.UDPConn, addr *net.UDPAddr) {
	terminationMessage := []byte("terminate")
	_, err := conn.WriteToUDP(terminationMessage, addr)
	if err != nil {
		fmt.Println("Error sending termination packet:", err)
	}
}

func displayTotalSummary(sentPackets, receivedPackets, receivedBytes, totalLatency, totalLatencyCount int64, startTime int64) {
	lossRate := float64(sentPackets-receivedPackets) / float64(sentPackets) * 100

	// Calculate elapsed time in seconds
	elapsedTime := float64(time.Now().UnixNano()-startTime) / 1e9

	// Calculate bandwidth in MiB/sec
	bandwidth := float64(receivedBytes) / elapsedTime // bytes/sec
	bandwidthMiB := bandwidth / (1024 * 1024) // convert to MiB/sec

	// Calculate average latency in ms
	averageLatency := float64(totalLatency) / float64(totalLatencyCount)

	fmt.Println("Total Summary:")
	fmt.Printf("Total sent packets: %d\n", sentPackets)
	fmt.Printf("Total received packets: %d\n", receivedPackets)
	fmt.Printf("Total packet loss rate: %.2f%%\n", lossRate)
	fmt.Printf("Total bandwidth throughput: %.2f MiB/sec\n", bandwidthMiB)
	fmt.Printf("Total average latency: %.2f ms\n", averageLatency)
	fmt.Println("-------------------------------")
}

