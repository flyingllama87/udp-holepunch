#include <iostream>
#include <string>
#include <cstring>
#include <thread>
#include <atomic>
#include <chrono>
#include <csignal>
#include <netdb.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <unistd.h>

std::atomic<int64_t> sentPackets(0);
std::atomic<int64_t> receivedPackets(0);
std::atomic<int64_t> receivedBytes(0);
std::atomic<int64_t> totalSentPackets(0);
std::atomic<int64_t> totalReceivedPackets(0);
std::atomic<int64_t> totalReceivedBytes(0);
std::atomic<int64_t> totalLatency(0);
std::atomic<int64_t> totalLatencyCount(0);
std::atomic<bool> terminateFlag(false);

int sockfd; // Make sockfd global so it can be accessed by the signal handler

void displayTotalSummary(int64_t startTime);
void signalHandler(int signum);
void sendTerminationPacket(int sockfd, const sockaddr_in& addr);
void sendPackets(int sockfd, const sockaddr_in& otherClientAddr, const std::string& clientKey, int pps);
void receivePackets(int sockfd, sockaddr_in& otherClientAddr);
void summaryLogger();

int main(int argc, char* argv[]) {
    if (argc < 5) {
        std::cerr << "Usage: client --server <server address> --key <client key> [--pps <packets per second>]\n";
        return 1;
    }

    std::string serverAddress = argv[2];
    std::string clientKey = argv[4];
    int pps = (argc == 7) ? std::stoi(argv[6]) : 10;

    signal(SIGINT, signalHandler);

    sockaddr_in localAddr{};
    localAddr.sin_family = AF_INET;
    localAddr.sin_port = htons(0);
    localAddr.sin_addr.s_addr = INADDR_ANY;

    sockfd = socket(AF_INET, SOCK_DGRAM, 0);
    if (sockfd < 0) {
        perror("socket creation failed");
        return 1;
    }

    if (bind(sockfd, (const sockaddr*)&localAddr, sizeof(localAddr)) < 0) {
        perror("bind failed");
        close(sockfd);
        return 1;
    }

    sockaddr_in serverAddr{};
    serverAddr.sin_family = AF_INET;
    serverAddr.sin_port = htons(8085);

    // Determine if the input is an IP address or a hostname
    if (inet_pton(AF_INET, serverAddress.c_str(), &serverAddr.sin_addr) <= 0) {
        // Not an IP address, perform DNS resolution
        struct addrinfo hints, *res;
        memset(&hints, 0, sizeof(hints));
        hints.ai_family = AF_INET;
        hints.ai_socktype = SOCK_DGRAM;
        int status = getaddrinfo(serverAddress.c_str(), "8085", &hints, &res);
        if (status != 0) {
            perror("getaddrinfo failed");
            return 1;
        }
        memcpy(&serverAddr, res->ai_addr, res->ai_addrlen);
        freeaddrinfo(res);
    }

    std::cout << "Sending client key to server..." << std::endl;
    int result = sendto(sockfd, clientKey.c_str(), clientKey.length(), 0, (const sockaddr*)&serverAddr, sizeof(serverAddr));
    if (result < 0) {
        perror("sendto failed");
        close(sockfd);
        return 1;
    }

    std::cout << "Waiting for response from server..." << std::endl;
    char buffer[1024];
    socklen_t len = sizeof(serverAddr);
    result = recvfrom(sockfd, buffer, 1024, 0, (sockaddr*)&serverAddr, &len);
    if (result < 0) {
        perror("recvfrom failed");
        close(sockfd);
        return 1;
    }

    std::string otherClientAddress(buffer);
    std::cout << "Received other client address: " << otherClientAddress << "\n";

    sockaddr_in otherClientAddr{};
    otherClientAddr.sin_family = AF_INET;
    otherClientAddr.sin_port = htons(std::stoi(otherClientAddress.substr(otherClientAddress.find(":") + 1)));
    inet_pton(AF_INET, otherClientAddress.substr(0, otherClientAddress.find(":")).c_str(), &otherClientAddr.sin_addr);

    int64_t startTime = std::chrono::high_resolution_clock::now().time_since_epoch().count();
    memcpy(buffer, &startTime, sizeof(startTime));
    sendto(sockfd, buffer, sizeof(startTime), 0, (const sockaddr*)&otherClientAddr, sizeof(otherClientAddr));

    result = recvfrom(sockfd, buffer, sizeof(startTime), 0, (sockaddr*)&otherClientAddr, &len);
    if (result < 0) {
        perror("recvfrom failed");
        close(sockfd);
        return 1;
    }
    startTime = *reinterpret_cast<int64_t*>(buffer);

    std::thread sendThread(sendPackets, sockfd, std::ref(otherClientAddr), std::ref(clientKey), pps);
    std::thread receiveThread(receivePackets, sockfd, std::ref(otherClientAddr));
    std::thread summaryThread(summaryLogger);

    sendThread.join();
    receiveThread.join();
    summaryThread.join();

    close(sockfd);
    return 0;
}

void sendPackets(int sockfd, const sockaddr_in& otherClientAddr, const std::string& clientKey, int pps) {
    while (!terminateFlag) {
        int64_t packetIndex = sentPackets.fetch_add(1);
        packetIndex = totalSentPackets.fetch_add(1);
        char message[1000];
        memset(message, 0, 1000);
        int64_t sendTime = std::chrono::high_resolution_clock::now().time_since_epoch().count();
        memcpy(message, &packetIndex, sizeof(packetIndex));
        memcpy(message + sizeof(packetIndex), &sendTime, sizeof(sendTime));
        snprintf(message + sizeof(packetIndex) + sizeof(sendTime), 1000 - sizeof(packetIndex) - sizeof(sendTime), "Packet %ld from %s", packetIndex, clientKey.c_str());

        int result = sendto(sockfd, message, 1000, 0, (const sockaddr*)&otherClientAddr, sizeof(otherClientAddr));
        if (result < 0) {
            perror("sendto failed");
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(1000 / pps));
    }
}

void receivePackets(int sockfd, sockaddr_in& otherClientAddr) {
    char buffer[1024];
    socklen_t len = sizeof(otherClientAddr);
    while (!terminateFlag) {
        int n = recvfrom(sockfd, buffer, 1024, 0, (sockaddr*)&otherClientAddr, &len);
        if (n < 0) {
            perror("recvfrom error");
            continue;
        }

        if (n == 9 && strncmp(buffer, "terminate", 9) == 0) {
            std::cout << "Received termination signal from the other client.\n";
            terminateFlag = true;
            displayTotalSummary(0);  // Pass startTime if needed for accurate elapsed time
            exit(0);
        }

        receivedPackets.fetch_add(1);
        receivedBytes.fetch_add(n);
        totalReceivedPackets.fetch_add(1);
        totalReceivedBytes.fetch_add(n);

        int64_t packetIndex = *reinterpret_cast<int64_t*>(buffer);
        int64_t sendTime = *reinterpret_cast<int64_t*>(buffer + sizeof(int64_t));
        int64_t latency = std::chrono::high_resolution_clock::now().time_since_epoch().count() - sendTime;
        int64_t latencyMs = latency / 1e6;  // convert to milliseconds
        totalLatency.fetch_add(latencyMs);
        totalLatencyCount.fetch_add(1);

        std::cout << "Received packet index: " << packetIndex << ", latency: " << latencyMs << " ms, sender IP: "
                  << inet_ntoa(otherClientAddr.sin_addr) << ", sender Port: " << ntohs(otherClientAddr.sin_port) << "\n";
    }
}

void summaryLogger() {
    while (!terminateFlag) {
        std::this_thread::sleep_for(std::chrono::seconds(5));

        int64_t sent = sentPackets.exchange(0);
        int64_t received = receivedPackets.exchange(0);
        int64_t bytes = receivedBytes.exchange(0);
        double lossRate = static_cast<double>(sent - received) / sent * 100;
        double bandwidth = static_cast<double>(bytes) / 5;  // bytes/5 seconds = bytes/second
        double bandwidthMiB = bandwidth / (1024 * 1024);  // convert to MiB/sec
        double averageLatency = static_cast<double>(totalLatency) / totalLatencyCount;

        std::cout << "Summary over the last 5 seconds:\n";
        std::cout << "Sent packets: " << sent << "\n";
        std::cout << "Received packets: " << received << "\n";
        std::cout << "Packet loss rate: " << lossRate << "%\n";
        std::cout << "Bandwidth throughput: " << bandwidthMiB << " MiB/sec\n";
        std::cout << "Average latency: " << averageLatency << " ms\n";
        std::cout << "-------------------------------\n";
    }
}

void displayTotalSummary(int64_t startTime) {
    int64_t endTime = std::chrono::high_resolution_clock::now().time_since_epoch().count();
    double elapsedTime = static_cast<double>(endTime - startTime) / 1e9;  // convert to seconds

    int64_t sent = totalSentPackets.load();
    int64_t received = totalReceivedPackets.load();
    int64_t bytes = totalReceivedBytes.load();
    double lossRate = static_cast<double>(sent - received) / sent * 100;
    double bandwidth = static_cast<double>(bytes) / elapsedTime;  // bytes/sec
    double bandwidthMiB = bandwidth / (1024 * 1024);  // convert to MiB/sec
    double averageLatency = static_cast<double>(totalLatency) / totalLatencyCount;

    std::cout << "Total Summary:\n";
    std::cout << "Total sent packets: " << sent << "\n";
    std::cout << "Total received packets: " << received << "\n";
    std::cout << "Total packet loss rate: " << lossRate << "%\n";
    std::cout << "Total bandwidth throughput: " << bandwidthMiB << " MiB/sec\n";
    std::cout << "Total average latency: " << averageLatency << " ms\n";
    std::cout << "-------------------------------\n";
}

void signalHandler(int signum) {
    terminateFlag = true;
    std::cerr << "Interrupt signal (" << signum << ") received.\n";
    displayTotalSummary(0); // Call displayTotalSummary here to print the summary before exiting
    close(sockfd); // Close the socket to properly terminate
    exit(0);
}

void sendTerminationPacket(int sockfd, const sockaddr_in& addr) {
    char terminationMessage[] = "terminate";
    sendto(sockfd, terminationMessage, strlen(terminationMessage), 0, (const sockaddr*)&addr, sizeof(addr));
}

