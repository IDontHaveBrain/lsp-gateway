package main

import (
	"fmt"
	"log"
	"time"
	"context"
	"lsp-gateway/tests/e2e/testutils"
)

func main() {
	fmt.Println("Health Check Optimization Performance Test")
	fmt.Println("==========================================")
	
	// Test optimized timeout manager
	testTimeoutManager()
	
	// Test optimized polling
	testOptimizedPolling()
	
	// Test fast health checker performance
	testFastHealthChecker()
}

func testTimeoutManager() {
	fmt.Println("\n1. Testing Optimized Timeout Manager:")
	
	start := time.Now()
	
	// Test profile selection with new simplified system
	tm := testutils.GetTimeoutManager()
	
	// Test various language/performance combinations
	languages := []testutils.Language{
		testutils.LanguageGo,
		testutils.LanguagePython, 
		testutils.LanguageJava,
		testutils.LanguageTypeScript,
	}
	
	performances := []testutils.SystemPerformance{
		testutils.SystemFast,
		testutils.SystemNormal,
		testutils.SystemSlow,
	}
	
	for _, lang := range languages {
		for _, perf := range performances {
			profile := tm.GetProfile(lang, perf)
			if profile == nil {
				log.Printf("Failed to get profile for %s-%s", lang, perf)
			}
		}
	}
	
	elapsed := time.Since(start)
	fmt.Printf("   Profile selection for 12 combinations: %v (avg: %v per selection)\n", 
		elapsed, elapsed/12)
	
	// Test context creation (should be much faster without tracking)
	start = time.Now()
	ctx := context.Background()
	profile := tm.GetProfile(testutils.LanguageGo, testutils.SystemFast)
	
	for i := 0; i < 100; i++ {
		testCtx, cancel := tm.CreateServerStartupContext(ctx, profile)
		cancel()
		_ = testCtx
	}
	
	elapsed = time.Since(start)
	fmt.Printf("   100 context creations: %v (avg: %v per context)\n", 
		elapsed, elapsed/100)
}

func testOptimizedPolling() {
	fmt.Println("\n2. Testing Optimized Polling Utilities:")
	
	// Test quick connectivity to a non-existent server (should fail fast)
	start := time.Now()
	err := testutils.QuickConnectivityTest("http://localhost:18080", 1*time.Second)
	elapsed := time.Since(start)
	
	fmt.Printf("   Quick connectivity test (expect failure): %v - %v\n", elapsed, err != nil)
	
	// Test optimized polling config creation
	start = time.Now()
	for i := 0; i < 1000; i++ {
		_ = testutils.OptimizedPollingConfig()
		_ = testutils.QuickPollingConfig()
		_ = testutils.DefaultPollingConfig()
	}
	elapsed = time.Since(start)
	fmt.Printf("   3000 config creations: %v (avg: %v per config)\n", 
		elapsed, elapsed/3000)
}

func testFastHealthChecker() {
	fmt.Println("\n3. Testing Fast Health Checker:")
	
	baseURL := "http://localhost:18080"
	
	start := time.Now()
	for i := 0; i < 10; i++ {
		fhc := testutils.NewFastHealthChecker(baseURL)
		fhc.Close()
	}
	elapsed := time.Since(start)
	fmt.Printf("   10 health checker creations: %v (avg: %v per creation)\n", 
		elapsed, elapsed/10)
	
	// Test quick TCP connectivity (should fail fast with optimized timeouts)
	fhc := testutils.NewFastHealthChecker(baseURL)
	defer fhc.Close()
	
	start = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err := fhc.QuickConnectivityCheck(ctx)
	elapsed = time.Since(start)
	
	fmt.Printf("   TCP connectivity check (expect failure): %v - %v\n", elapsed, err != nil)
	fmt.Printf("   Expected improvement: TCP timeout reduced from 1000ms to 500ms\n")
}