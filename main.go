package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Check for version flag
	if os.Args[1] == "-version" || os.Args[1] == "--version" {
		fmt.Printf("pokego version %s\n", version)
		os.Exit(0)
	}

	// Parse subcommand
	switch os.Args[1] {
	case "http":
		httpCmd := flag.NewFlagSet("http", flag.ExitOnError)
		url := httpCmd.String("url", "", "URL to send POST request to (required)")
		timeout := httpCmd.Duration("timeout", 30*time.Second, "Request timeout")
		verbose := httpCmd.Bool("verbose", false, "Enable verbose output")

		httpCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: pokego http -url=<url> [options]\n\n")
			fmt.Fprintf(os.Stderr, "Send POST request to reload application\n\n")
			fmt.Fprintf(os.Stderr, "Options:\n")
			httpCmd.PrintDefaults()
			fmt.Fprintf(os.Stderr, "\nExamples:\n")
			fmt.Fprintf(os.Stderr, "  pokego http -url=http://localhost:8080/-/reload\n")
			fmt.Fprintf(os.Stderr, "  pokego http -url=http://localhost:9090/-/reload\n")
		}

		if err := httpCmd.Parse(os.Args[2:]); err != nil {
			os.Exit(1)
		}

		if *url == "" {
			httpCmd.Usage()
			os.Exit(1)
		}

		if err := doHTTPRequest(*url, "POST", "", *timeout, *verbose); err != nil {
			log.Fatalf("HTTP request failed: %v", err)
		}

	case "sighup":
		sighupCmd := flag.NewFlagSet("sighup", flag.ExitOnError)
		name := sighupCmd.String("name", "", "Process name to send SIGHUP to (required)")
		all := sighupCmd.Bool("all", false, "Send signal to all matching processes")
		verbose := sighupCmd.Bool("verbose", false, "Enable verbose output")

		sighupCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: pokego sighup -name=<process-name> [options]\n\n")
			fmt.Fprintf(os.Stderr, "Send SIGHUP signal to process by name\n\n")
			fmt.Fprintf(os.Stderr, "Options:\n")
			sighupCmd.PrintDefaults()
			fmt.Fprintf(os.Stderr, "\nExamples:\n")
			fmt.Fprintf(os.Stderr, "  pokego sighup -name=myapp\n")
			fmt.Fprintf(os.Stderr, "  pokego sighup -name=custom-exporter -all\n")
		}

		if err := sighupCmd.Parse(os.Args[2:]); err != nil {
			os.Exit(1)
		}

		if *name == "" {
			sighupCmd.Usage()
			os.Exit(1)
		}

		if err := doSIGHUP(*name, *all, *verbose); err != nil {
			log.Fatalf("SIGHUP failed: %v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "pokego - Poke your processes to reload them\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  pokego <command> [options]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  http     Send POST request to reload endpoint\n")
	fmt.Fprintf(os.Stderr, "  sighup   Send SIGHUP signal to process by name\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	fmt.Fprintf(os.Stderr, "  -version  Show version information\n\n")
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  pokego http -url=http://localhost:8080/-/reload\n")
	fmt.Fprintf(os.Stderr, "  pokego sighup -name=myapp\n\n")
	fmt.Fprintf(os.Stderr, "Use 'pokego <command> -h' for more information about a command.\n")
}

func doHTTPRequest(url, method, body string, timeout time.Duration, verbose bool) error {
	client := &http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if verbose {
		log.Printf("Sending %s request to %s", method, url)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if verbose && len(respBody) > 0 {
		log.Printf("Response body: %s", string(respBody))
	}

	// Check status code
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("Successfully poked %s (status: %d)", url, resp.StatusCode)
	} else {
		return fmt.Errorf("server returned error status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func doSIGHUP(processName string, all bool, verbose bool) error {
	// Get all processes
	processes, err := process.Processes()
	if err != nil {
		return fmt.Errorf("failed to list processes: %w", err)
	}

	var foundPIDs []int32
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}

		// Match process name (exact match or contains)
		if name == processName || strings.Contains(name, processName) {
			foundPIDs = append(foundPIDs, p.Pid)
			if verbose {
				cmdline, _ := p.Cmdline()
				log.Printf("Found process: PID=%d, Name=%s, Cmdline=%s", p.Pid, name, cmdline)
			}
			if !all {
				break // Only process first match unless -all is specified
			}
		}
	}

	if len(foundPIDs) == 0 {
		return fmt.Errorf("no process found with name %q", processName)
	}

	// Send SIGHUP to found processes
	successCount := 0
	var lastErr error

	for _, pid := range foundPIDs {
		proc, err := os.FindProcess(int(pid))
		if err != nil {
			lastErr = fmt.Errorf("failed to find process %d: %w", pid, err)
			log.Printf("Warning: %v", lastErr)
			continue
		}

		if err := proc.Signal(syscall.SIGHUP); err != nil {
			lastErr = fmt.Errorf("failed to send SIGHUP to PID %d: %w", pid, err)
			log.Printf("Warning: %v", lastErr)
			continue
		}

		log.Printf("Successfully poked process %s (PID: %d) with SIGHUP", processName, pid)
		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("failed to send signal to any process: %v", lastErr)
	}

	if all && successCount < len(foundPIDs) {
		log.Printf("Warning: Only sent signal to %d out of %d processes", successCount, len(foundPIDs))
	}

	return nil
}

// Helper function to truncate long strings for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
