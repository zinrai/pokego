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

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Check for version flag
	if os.Args[1] == "-version" || os.Args[1] == "--version" {
		fmt.Printf("pokego version %s (commit %s, built %s)\n", version, commit, date)
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
		verbose := sighupCmd.Bool("verbose", false, "Enable verbose output")

		sighupCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: pokego sighup -name=<process-name> [options]\n\n")
			fmt.Fprintf(os.Stderr, "Send SIGHUP signal to process by name\n\n")
			fmt.Fprintf(os.Stderr, "Options:\n")
			sighupCmd.PrintDefaults()
		}

		if err := sighupCmd.Parse(os.Args[2:]); err != nil {
			os.Exit(1)
		}

		if *name == "" {
			sighupCmd.Usage()
			os.Exit(1)
		}

		if err := doSIGHUP(*name, *verbose); err != nil {
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

type procInfo struct {
	pid  int32
	ppid int32
	name string
}

func doSIGHUP(processName string, verbose bool) error {
	processes, err := process.Processes()
	if err != nil {
		return fmt.Errorf("failed to list processes: %w", err)
	}

	// Collect every process whose name starts with the given prefix.
	var matched []procInfo
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if !strings.HasPrefix(name, processName) {
			continue
		}
		ppid, err := p.Ppid()
		if err != nil {
			continue
		}
		matched = append(matched, procInfo{pid: p.Pid, ppid: ppid, name: name})

		if verbose {
			cmdline, _ := p.Cmdline()
			log.Printf("Matched process: PID=%d, PPID=%d, Name=%s, Cmdline=%s", p.Pid, ppid, name, cmdline)
		}
	}

	if len(matched) == 0 {
		return fmt.Errorf("no process found with name %q", processName)
	}

	// Prefork servers share a name between master and workers, so a name match
	// alone can land on a worker. A matched process is a master when its parent
	// is not itself a matched process. Workers are parented by the master and
	// fall out here. A process tree has no cycles, so at least one master
	// always remains.
	matchedPIDs := make(map[int32]bool, len(matched))
	for _, m := range matched {
		matchedPIDs[m.pid] = true
	}

	var masters []procInfo
	for _, m := range matched {
		isWorker := matchedPIDs[m.ppid]
		if isWorker && verbose {
			log.Printf("Skipping worker: PID=%d (parent PID %d is also a match)", m.pid, m.ppid)
		}
		if isWorker {
			continue
		}
		masters = append(masters, m)
	}

	// A running service has a single master, so a match that resolves to more
	// than one master spans unrelated process trees (separate instances, or an
	// unrelated daemon sharing the prefix). Refuse rather than signal a process
	// the caller did not mean.
	if len(masters) > 1 {
		return fmt.Errorf("prefix %q matched multiple master processes (%s), narrow -name to one", processName, formatProcs(masters))
	}

	// Exactly one master remains. Signal it, never a worker.
	master := masters[0]
	proc, err := os.FindProcess(int(master.pid))
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", master.pid, err)
	}
	if err := proc.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("failed to send SIGHUP to PID %d: %w", master.pid, err)
	}
	log.Printf("Successfully poked master %s (PID: %d) with SIGHUP", master.name, master.pid)

	return nil
}

func formatProcs(procs []procInfo) string {
	parts := make([]string, len(procs))
	for i, p := range procs {
		parts[i] = fmt.Sprintf("%s (PID %d)", p.name, p.pid)
	}
	return strings.Join(parts, ", ")
}
