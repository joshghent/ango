package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	numRequests = 1000 // Total number of requests to send
	concurrency = 100  // Number of concurrent requests
	url         = "https://ango-73r94.ondigitalocean.app/api/get-code"
)

var (
	jsonData = []byte(`{
		"batchid": "11111111-1111-1111-1111-111111111111",
		"clientid": "217be7c8-679c-4e08-bffc-db3451bdcdbf",
		"customerid": "fba9230a-a521-430e-aaf8-8aefbf588071"
	}`)
	codeMutex    sync.Mutex
	codes        = make(map[string]struct{})
	times        []time.Duration
	timeMutex    sync.Mutex
	failedCount  int
	failedMutex  sync.Mutex
	successCount int
	successMutex sync.Mutex
)

func main() {
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numRequests/concurrency; j++ {
				startTime := time.Now()

				resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
				if err != nil {
					failedMutex.Lock()
					failedCount++
					failedMutex.Unlock()
					fmt.Printf("Request failed: %v\n", err)
					continue
				}

				var code string
				_, err = fmt.Fscan(resp.Body, &code)
				resp.Body.Close()
				if err != nil {
					failedMutex.Lock()
					failedCount++
					failedMutex.Unlock()
					fmt.Printf("Failed to read response body: %v\n", err)
					continue
				}

				if resp.StatusCode != http.StatusOK {
					failedMutex.Lock()
					failedCount++
					failedMutex.Unlock()
					fmt.Printf("Unexpected status code: %d\n", resp.StatusCode)
					continue
				}

				timeTaken := time.Since(startTime)

				timeMutex.Lock()
				times = append(times, timeTaken)
				timeMutex.Unlock()

				successMutex.Lock()
				successCount++
				successMutex.Unlock()

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
	duration := time.Since(start)

	// Sorting the times to calculate percentiles
	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	// Calculating percentiles
	getPercentile := func(p int) time.Duration {
		index := (p * len(times)) / 100
		if index >= len(times) {
			index = len(times) - 1
		}
		return times[index]
	}

	fmt.Printf("Completed %d requests in %v\n", numRequests, duration)
	fmt.Printf("Total unique codes: %d\n", len(codes))
	fmt.Printf("Total successful requests: %d\n", successCount)
	fmt.Printf("Total failed requests: %d\n", failedCount)
	fmt.Printf("Average time per successful request: %v\n", duration/time.Duration(successCount))
	fmt.Printf("50th percentile time: %v\n", getPercentile(50))
	fmt.Printf("75th percentile time: %v\n", getPercentile(75))
	fmt.Printf("90th percentile time: %v\n", getPercentile(90))
	fmt.Printf("95th percentile time: %v\n", getPercentile(95))
	fmt.Printf("99th percentile time: %v\n", getPercentile(99))
}
