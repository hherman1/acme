// written by chatgpt
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

// Contributor holds email and count of lines contributed
type Contributor struct {
	Email      string
	LineCount  int
	Percentage float64
}

func main() {
	// Read file names from stdin
	files := readInputFiles()

	if len(files) == 0 {
		fmt.Println("No input files provided.")
		return
	}

	// Process files concurrently
	contributions := processFilesConcurrently(files)

	if len(contributions) == 0 {
		fmt.Println("No authors found. (Either no files processed or no valid git blame data.)")
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

// processFilesConcurrently runs git blame concurrently on multiple files
func processFilesConcurrently(files []string) map[string]int {
	var wg sync.WaitGroup
	contributions := make(map[string]int)
	mu := sync.Mutex{}
	ch := make(chan map[string]int, len(files))

	// Process each file concurrently
	for _, file := range files {
		wg.Add(1)
		go func(file string) {
			defer wg.Done()
			if stats := processFile(file); len(stats) > 0 {
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

// processFile runs 'git blame' on a single file and counts line contributions per author email
func processFile(file string) map[string]int {
	cmd := exec.Command("git", "blame", "--line-porcelain", file)
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to process file '%s': %v\n", file, err)
		return nil
	}

	// Parse the output
	return parseGitBlameOutput(output)
}

// parseGitBlameOutput extracts author emails and counts occurrences
func parseGitBlameOutput(output []byte) map[string]int {
	contributions := make(map[string]int)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "author-mail <") {
			email := strings.TrimPrefix(line, "author-mail <")
			email = strings.TrimSuffix(email, ">")
			contributions[email]++
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
