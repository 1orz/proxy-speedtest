// Package tester provides proxy speed testing functionality
package tester

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// TestMode defines what tests to run
type TestMode int

const (
	TestModeAll       TestMode = iota // Run both ping and speed test
	TestModePingOnly                  // Run only ping test
	TestModeSpeedOnly                 // Run only speed test
)

// Options configures the tester
type Options struct {
	Concurrency  int           // Number of concurrent tests
	Timeout      time.Duration // Timeout for each test
	DownloadURL  string        // URL to download for speed test
	DownloadSize string        // Download size preset (10mb, 100mb, etc.)
	Mode         TestMode      // Test mode
}

// DefaultOptions returns default tester options
func DefaultOptions() *Options {
	return &Options{
		Concurrency:  2,
		Timeout:      15 * time.Second,
		DownloadURL:  "",
		DownloadSize: "10mb",
		Mode:         TestModeAll,
	}
}

// Result represents the test result for a single proxy
type Result struct {
	Config   *xray.ProxyConfig // Proxy configuration
	Index    int               // Original index in the list
	Ping     int64             // Latency in milliseconds (0 = failed)
	AvgSpeed int64             // Average download speed in bytes/s
	MaxSpeed int64             // Maximum download speed in bytes/s
	Traffic  int64             // Total traffic downloaded in bytes
	Error    error             // Error if test failed
	Success  bool              // Whether the proxy is working
}

// Tester performs speed tests on proxies
type Tester struct {
	options *Options
}

// New creates a new Tester with the given options
func New(options *Options) *Tester {
	if options == nil {
		options = DefaultOptions()
	}
	if options.Concurrency < 1 {
		options.Concurrency = 1
	}
	if options.Timeout < time.Second {
		options.Timeout = 15 * time.Second
	}
	return &Tester{options: options}
}

// Test runs tests on all provided proxy configs and returns results via channel
func (t *Tester) Test(ctx context.Context, configs []*xray.ProxyConfig) <-chan *Result {
	resultChan := make(chan *Result, len(configs))

	go func() {
		defer close(resultChan)

		var wg sync.WaitGroup
		semaphore := make(chan struct{}, t.options.Concurrency)

		for i, config := range configs {
			select {
			case <-ctx.Done():
				return
			case semaphore <- struct{}{}:
			}

			wg.Add(1)
			go func(index int, cfg *xray.ProxyConfig) {
				defer wg.Done()
				defer func() { <-semaphore }()

				result := t.testOne(ctx, index, cfg)
				select {
				case resultChan <- result:
				case <-ctx.Done():
				}
			}(i, config)
		}

		wg.Wait()
	}()

	return resultChan
}

// TestSync runs tests synchronously and returns all results
func (t *Tester) TestSync(ctx context.Context, configs []*xray.ProxyConfig) []*Result {
	results := make([]*Result, len(configs))
	resultChan := t.Test(ctx, configs)

	for result := range resultChan {
		results[result.Index] = result
	}

	return results
}

// testOne tests a single proxy
func (t *Tester) testOne(ctx context.Context, index int, config *xray.ProxyConfig) *Result {
	result := &Result{
		Config: config,
		Index:  index,
	}

	// Create dialer
	dialer, err := xray.NewDialer(config)
	if err != nil {
		result.Error = fmt.Errorf("create dialer: %w", err)
		return result
	}
	defer dialer.Close()

	// Ping test
	if t.options.Mode != TestModeSpeedOnly {
		ping, err := t.pingTest(ctx, dialer)
		if err != nil {
			result.Error = fmt.Errorf("ping test: %w", err)
			result.Ping = 0
			return result
		}
		result.Ping = ping
		result.Success = true
	}

	// Speed test
	if t.options.Mode != TestModePingOnly {
		avgSpeed, maxSpeed, traffic, err := t.speedTest(ctx, dialer)
		if err != nil {
			// Ping succeeded but speed test failed
			if result.Success {
				result.Error = fmt.Errorf("speed test: %w", err)
			}
		} else {
			result.AvgSpeed = avgSpeed
			result.MaxSpeed = maxSpeed
			result.Traffic = traffic
			result.Success = true
		}
	}

	return result
}

// Options returns the tester options
func (t *Tester) Options() *Options {
	return t.options
}

