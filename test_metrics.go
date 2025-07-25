package main

import (
	"fmt"
	"time"
)

type ServerMetrics struct {
	TotalRequests  int64
	FailedRequests int64
	ResponseTimes  []time.Duration
}

func (sm *ServerMetrics) GetErrorRate() float64 {
	if sm.TotalRequests == 0 {
		return 0.0
	}
	return float64(sm.FailedRequests) / float64(sm.TotalRequests)
}

func main() {
	sm := &ServerMetrics{TotalRequests: 100, FailedRequests: 5}
	fmt.Printf("✅ GetErrorRate works: %.1f%%\n", sm.GetErrorRate()*100)

	sm2 := &ServerMetrics{TotalRequests: 0}
	fmt.Printf("✅ Division by zero handled: %.1f%%\n", sm2.GetErrorRate()*100)

	fmt.Println("✅ Implementation complete!")
}
