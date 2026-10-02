package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Contributor holds email and count of lines contributed
type Contributor struct {
	Email      string
	LineCount  int
	Percentage float64
}

// Time format expected in input argument (YYYY-MM-DD)
const timeFormat = "2006-01-02"

func main() {
	// Check for the required argument (date filter)
	if len(os.Args) < 3 {
		fmt.Println("Usage: ./author_histogram <YYYY-MM-DD>")
		os.Exit(1)
	}

	// Parse the cutoff date
	startDate, err := time.Parse(timeFormat, os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid date format %q. Expected YYYY-MM-DD: %v", os.Args[1], err)
		os.Exit(1)
	}

	endDate, err := time.Parse(timeFormat, os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid date format %q. Expected YYYY-MM-DD: %v", os.Args[1], err)
		os.Exit(1)
	}

	// Read file names from stdin
	files := readInputFiles()

	if len(files) == 0 {
		fmt.Println("No input files provided.")
		return
	}

	// Process files concurrently
	contributions := processFilesConcurrently(files, startDate, endDate)

	if len(contributions) == 0 {
		fmt.Println("No authors found (either no files processed or no valid git blame data).")
		return
	}

	// Compute total line count and percentages
	totalLines := 0
	for _, count := range contributions {
		totalLines += count
	}

	var contributors []Contributor
	for email, count := range contributions {
		contributors = append(contributors, Contributor{
			Email:      email,
			LineCount:  count,
			Percentage: (float64(count) / float64(totalLines)) * 100,
		})
	}

	// Sort contributors by line count (descending)
	sort.Slice(contributors, func(i, j int) bool {
		return contributors[i].LineCount > contributors[j].LineCount
	})

	// Print formatted results
	printResults(contributors, totalLines)
}

// readInputFiles reads filenames from stdin and returns them as a slice
func readInputFiles() []string {
	var files []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		file := strings.TrimSpace(scanner.Text())
		if file != "" {
			files = append(files, file)
		}
	}
	return files
}

// processFilesConcurrently runs git blame concurrently on multiple files with a date filter
func processFilesConcurrently(files []string, startDate time.Time, endDate time.Time) map[string]int {
	var wg sync.WaitGroup
	contributions := make(map[string]int)
	mu := sync.Mutex{}
	ch := make(chan map[string]int, len(files))

	// Process each file concurrently
	for _, file := range files {
		wg.Add(1)
		go func(file string) {
			defer wg.Done()
			if stats := processFile(file, startDate, endDate); len(stats) > 0 {
				ch <- stats
			}
		}(file)
	}

	// Close the channel once all goroutines are done
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Aggregate results from all goroutines
	for result := range ch {
		mu.Lock()
		for email, count := range result {
			contributions[email] += count
		}
		mu.Unlock()
	}

	return contributions
}

// processFile runs 'git blame' on a single file and filters by date
func processFile(file string, startDate time.Time, endDate time.Time) map[string]int {
	cmd := exec.Command("git", "blame", "--line-porcelain", file)
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to process file '%s': %v\n", file, err)
		return nil
	}

	// Parse the output and filter by date
	return parseGitBlameOutput(output, startDate, endDate)
}

// parseGitBlameOutput extracts author emails and counts occurrences, filtering by date
func parseGitBlameOutput(output []byte, startDate time.Time, endDate time.Time) map[string]int {
	contributions := make(map[string]int)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	var currentEmail string
	var currentTimestamp int64

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "author-mail <") {
			currentEmail = strings.TrimPrefix(line, "author-mail <")
			currentEmail = strings.TrimSuffix(currentEmail, ">")
		} else if strings.HasPrefix(line, "author-time ") {
			timestampStr := strings.TrimPrefix(line, "author-time ")
			timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
			if err == nil {
				currentTimestamp = timestamp
			}
			// Process a new blame entry (this always comes after mail)
			if currentEmail != "" && currentTimestamp != 0 {
				commitDate := time.Unix(currentTimestamp, 0)
				geStart := commitDate.After(startDate) || commitDate.Equal(startDate)
				leEnd := commitDate.Before(endDate) || commitDate.Equal(endDate)
				if geStart && leEnd {
					contributions[currentEmail]++
				}
			}
		}
	}

	return contributions
}

// printResults formats and prints the contributor data
func printResults(contributors []Contributor, totalLines int) {
	fmt.Printf("%-30s %10s %10s\n", "Contributor", "Count", "Percentage")
	fmt.Printf("%-30s %10s %10s\n", "-----------", "-----", "----------")

	for _, c := range contributors {
		fmt.Printf("%-30s %10d %9.2f%%\n", c.Email, c.LineCount, c.Percentage)
	}

	fmt.Printf("\n%-30s %10d %9.2f%%\n", "Total", totalLines, 100.00)
}
