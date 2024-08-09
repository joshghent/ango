package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	numRequests = 1000 // Total number of requests to send
	concurrency = 100  // Number of concurrent requests
	url         = "http://localhost:3000/api/get-code"
)

var (
	jsonData = []byte(`{
		"batchid": "11111111-1111-1111-1111-111111111111",
		"clientid": "217be7c8-679c-4e08-bffc-db3451bdcdbf",
		"customerid": "fba9230a-a521-430e-aaf8-8aefbf588071"
	}`)
	codeMutex sync.Mutex
	codes     = make(map[string]struct{})
)

func main() {
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numRequests/concurrency; j++ {
				resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
				if err != nil {
					fmt.Printf("Request failed: %v\n", err)
					continue
				}

				var code string
				_, err = fmt.Fscan(resp.Body, &code)
				resp.Body.Close()
				if err != nil {
					fmt.Printf("Failed to read response body: %v\n", err)
					continue
				}

				if resp.StatusCode != http.StatusOK {
					fmt.Printf("Unexpected status code: %d\n", resp.StatusCode)
					continue
				}

				codeMutex.Lock()
				if _, exists := codes[code]; exists {
					fmt.Printf("Duplicate code detected: %s\n", code)
				} else {
					codes[code] = struct{}{}
				}
				codeMutex.Unlock()
			}
		}()
	}

	wg.Wait()
	fmt.Printf("Completed %d requests in %v\n", numRequests, time.Since(start))
	fmt.Printf("Total unique codes: %d\n", len(codes))
}
