# UDP Hole Punching test apps

Test utilities for [UDP hole punching](https://en.wikipedia.org/wiki/UDP_hole_punching) to establish a P2P connection behind NAT routers.

First, a lightweight server sets up the initial connection between the clients and then the clients interact with each other directly.

Notes:

- The latency calculation depend on the system time being very accurate. Suggest sync to the same NTP server first.
- Server runs on TCP 8085 by default.

### Building
- Needs golang installed for client + server golang build. Clang/LLVM and/or GCC can be used for C++ clients. 

- `build-cpp-client.sh` - Build the CPP clients with gcc & clang.
- `build-go.sh` - Build both linux and windows golang server and client.


### Server
```
$ ./server-go
Client connected: testkey (IP: 12.42.53.11 Port: 48678)
Client connected: testkey (IP: 110.12.151.5, Port: 15557)
```

### Client
- packets per second (PPS) can be controller with the --pps option.
```
./client-go --server <SERVER_HOSTNAME_OR_IP> --key test2
Received other client address: <OTHER_CLIENT_IP>:15605
Received packet index: 1, message: Packet 1 from test2, latency: 68 ms, sender IP: 12.42.53.1, sender Port: 15605
...<SNIP>...
Received packet index: 45, message: Packet 45 from test2, latency: 57 ms, sender IP: 12.42.53.1, sender Port: 15605
Summary over the last 5 seconds:
Sent packets: 50
Received packets: 46
Packet loss rate: 8%
Bandwidth throughput: 0.0087738 MiB/sec
Average latency: 41.5789 ms
-------------------------------
...<SNIP>...
^CInterrupt signal (2) received.
Total Summary:
Total sent packets: 128
Total received packets: 122
Total packet loss rate: 4.6875%
Total bandwidth throughput: 0.01 MiB/sec
Total average latency: 48.6066 ms
-------------------------------
```


### Simple client
- Simply sends a "ping" message between the clients every second.
```
./client-simple <SERVER_HOSTNAME_OR_IP> test2
Received other client address: 12.42.53.1:15593
Received: Ping from test2
Received: Ping from test2
Received: Ping from test2
Received: Ping from test2
Received: Ping from test2
Received: Ping from test2
Received: Ping from test2
```