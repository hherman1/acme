package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type DiffLineType int

const (
	LineContext DiffLineType = iota
	LineDeleted
	LineAdded
)

type DiffLine struct {
	Type    DiffLineType
	OldLine int
	NewLine int
}

type CommentTask struct {
	Path           string
	LineSpec       string
	Side           string
	Body           string
	ReviewFileLine int
}

type PRDetails struct {
	Host     string
	BaseRepo string
}

type commentResult struct {
	task CommentTask
	err  error
}

var hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <reviewfile> [<PR number>]\n", os.Args[0])
		os.Exit(1)
	}

	reviewFilePath := relativizePath(os.Args[1])
	var prNumber string

	if len(os.Args) >= 3 {
		prNumber = os.Args[2]
	} else {
		var err error
		prNumber, err = getPRNumberFromBranch()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error determining PR number: %v\n", err)
			os.Exit(1)
		}
	}

	comments, errs := parseReviewFile(reviewFilePath)
	if len(errs) > 0 {
		for _, errStr := range errs {
			fmt.Fprintln(os.Stderr, errStr)
		}
		os.Exit(1)
	}

	if len(comments) == 0 {
		fmt.Println("No comments found to post.")
		return
	}

	prDetails, err := getPRDetails(prNumber)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving PR details: %v\n", err)
		os.Exit(1)
	}

	localSHA, err := getLocalHeadCommitSHA()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving local git HEAD SHA: %v\n", err)
		os.Exit(1)
	}

	remoteSHA, err := getRemotePRCommitSHA(prNumber)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving remote PR HEAD SHA: %v\n", err)
		os.Exit(1)
	}

	if localSHA != remoteSHA {
		fmt.Fprintf(os.Stderr, "Note: Local HEAD (%s) differs from remote PR HEAD (%s).\n"+
			"Posting comments against remote PR HEAD.\n\n",
			localSHA[:7], remoteSHA[:7])
	}

	runConcurrently(reviewFilePath, prDetails, prNumber, remoteSHA, comments)
}

func relativizePath(p string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(cwd, p)
	if err != nil {
		return p
	}
	return rel
}

func getPRDetails(prNumber string) (PRDetails, error) {
	cmd := exec.Command("gh", "pr", "view", prNumber, "--json", "url", "-q", ".url")
	out, err := cmd.Output()
	if err != nil {
		return PRDetails{}, fmt.Errorf("failed to retrieve PR URL: %w", err)
	}
	rawURL := strings.TrimSpace(string(out))

	u, err := url.Parse(rawURL)
	if err != nil {
		return PRDetails{}, fmt.Errorf("invalid PR URL %q: %w", rawURL, err)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, part := range parts {
		if part == "pull" && i >= 2 {
			return PRDetails{
				Host:     u.Host,
				BaseRepo: parts[i-2] + "/" + parts[i-1],
			}, nil
		}
	}
	return PRDetails{}, fmt.Errorf("could not parse owner/repo from PR URL %q", rawURL)
}

func getPRNumberFromBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git branch: %w", err)
	}

	branch := strings.TrimSpace(string(out))
	parts := strings.Split(branch, "/")
	if len(parts) < 2 || parts[0] == "" {
		return "", fmt.Errorf("branch %q is not in expected format <pr number>/<remote branch name>", branch)
	}

	if _, err := strconv.Atoi(parts[0]); err != nil {
		return "", fmt.Errorf("branch prefix %q in %q is not a valid numeric PR number", parts[0], branch)
	}

	return parts[0], nil
}

func getLocalHeadCommitSHA() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get local HEAD commit SHA: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func getRemotePRCommitSHA(prNumber string) (string, error) {
	cmd := exec.Command("gh", "pr", "view", prNumber, "--json", "headRefOid", "-q", ".headRefOid")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve remote PR head commit SHA: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func parseReviewFile(filePath string) ([]CommentTask, []string) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, []string{fmt.Sprintf("Failed to open file %s: %v", filePath, err)}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	const maxCapacity = 16 * 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	var errs []string
	var comments []CommentTask

	var currentPath string
	var hunkLines []DiffLine
	var currentOldLine int
	var currentNewLine int
	rangeStartIndex := -1

	fileLineNum := 0
	inMultiline := false
	var multilineBody []string
	multilineStartLineNum := 0

	for scanner.Scan() {
		fileLineNum++
		line := scanner.Text()

		if inMultiline {
			if strings.HasPrefix(line, ">") {
				afterGt := strings.TrimPrefix(line, ">")
				if strings.TrimSpace(afterGt) == "" {
					inMultiline = false
					bodyText := strings.Join(multilineBody, "\n")

					comment, errStr := buildCommentTask(filePath, multilineStartLineNum, currentPath, hunkLines, rangeStartIndex, bodyText)
					if errStr != "" {
						errs = append(errs, errStr)
					} else {
						comments = append(comments, comment)
					}

					rangeStartIndex = -1
					multilineBody = nil
					continue
				} else {
					errs = append(errs, fmt.Sprintf("%s:%d: comment line in multiline block cannot start with '>'", filePath, fileLineNum))
					inMultiline = false
					multilineBody = nil
					rangeStartIndex = -1
					continue
				}
			}
			multilineBody = append(multilineBody, line)
			continue
		}

		if strings.HasPrefix(line, "--- ") {
			if p := extractPathFromHeader(line, "--- "); p != "" {
				currentPath = p
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			if p := extractPathFromHeader(line, "+++ "); p != "" {
				currentPath = p
			}
			hunkLines = nil
			rangeStartIndex = -1
			continue
		}
		if strings.HasPrefix(line, "diff --git ") {
			hunkLines = nil
			rangeStartIndex = -1
			continue
		}

		if strings.HasPrefix(line, "@@") {
			matches := hunkHeaderRegex.FindStringSubmatch(line)
			if len(matches) >= 4 {
				currentOldLine, _ = strconv.Atoi(matches[1])
				currentNewLine, _ = strconv.Atoi(matches[3])
			}
			hunkLines = nil
			rangeStartIndex = -1
			continue
		}

		if strings.TrimSpace(line) == "<" {
			rangeStartIndex = len(hunkLines)
			continue
		}

		if strings.HasPrefix(line, ">") {
			afterGt := strings.TrimPrefix(line, ">")
			if strings.TrimSpace(afterGt) == "" {
				inMultiline = true
				multilineStartLineNum = fileLineNum
				multilineBody = nil
				continue
			} else {
				commentText := afterGt
				if strings.HasPrefix(commentText, " ") {
					commentText = commentText[1:]
				}

				comment, errStr := buildCommentTask(filePath, fileLineNum, currentPath, hunkLines, rangeStartIndex, commentText)
				if errStr != "" {
					errs = append(errs, errStr)
				} else {
					comments = append(comments, comment)
				}

				rangeStartIndex = -1
				continue
			}
		}

		if len(line) > 0 {
			switch line[0] {
			case ' ':
				hunkLines = append(hunkLines, DiffLine{
					Type:    LineContext,
					OldLine: currentOldLine,
					NewLine: currentNewLine,
				})
				currentOldLine++
				currentNewLine++
			case '-':
				hunkLines = append(hunkLines, DiffLine{
					Type:    LineDeleted,
					OldLine: currentOldLine,
					NewLine: 0,
				})
				currentOldLine++
			case '+':
				hunkLines = append(hunkLines, DiffLine{
					Type:    LineAdded,
					OldLine: 0,
					NewLine: currentNewLine,
				})
				currentNewLine++
			}
		}
	}

	if inMultiline {
		errs = append(errs, fmt.Sprintf("%s:%d: unclosed multiline comment", filePath, multilineStartLineNum))
	}

	if err := scanner.Err(); err != nil {
		errs = append(errs, fmt.Sprintf("Error reading file %s: %v", filePath, err))
	}

	return comments, errs
}

func extractPathFromHeader(line, prefix string) string {
	p := strings.TrimPrefix(line, prefix)
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "\"") && strings.HasSuffix(p, "\"") && len(p) >= 2 {
		p = p[1 : len(p)-1]
	}
	if p == "/dev/null" {
		return ""
	}
	if strings.HasPrefix(p, "a/") {
		return strings.TrimPrefix(p, "a/")
	}
	if strings.HasPrefix(p, "b/") {
		return strings.TrimPrefix(p, "b/")
	}
	return p
}

func buildCommentTask(filePath string, lineNum int, currentPath string, hunkLines []DiffLine, rangeStartIndex int, body string) (CommentTask, string) {
	if len(hunkLines) == 0 {
		return CommentTask{}, fmt.Sprintf("%s:%d: comment marker found before any diff lines in hunk", filePath, lineNum)
	}

	var selected []DiffLine
	if rangeStartIndex != -1 {
		if rangeStartIndex >= len(hunkLines) {
			return CommentTask{}, fmt.Sprintf("%s:%d: selection marker '<' range is empty", filePath, lineNum)
		}
		selected = hunkLines[rangeStartIndex:]
	} else {
		selected = hunkLines[len(hunkLines)-1:]
	}

	hasDeleted := false
	hasAdded := false

	for _, l := range selected {
		if l.Type == LineDeleted {
			hasDeleted = true
		} else if l.Type == LineAdded {
			hasAdded = true
		}
	}

	if hasDeleted && hasAdded {
		return CommentTask{}, fmt.Sprintf("%s:%d: comments cannot include added and deleted lines", filePath, lineNum)
	}

	var side string
	var lineSpec string

	if hasDeleted {
		side = "LEFT"
		startOld := selected[0].OldLine
		endOld := selected[len(selected)-1].OldLine
		if startOld == endOld {
			lineSpec = strconv.Itoa(startOld)
		} else {
			lineSpec = fmt.Sprintf("%d-%d", startOld, endOld)
		}
	} else {
		side = "RIGHT"
		startNew := selected[0].NewLine
		endNew := selected[len(selected)-1].NewLine
		if startNew == endNew {
			lineSpec = strconv.Itoa(startNew)
		} else {
			lineSpec = fmt.Sprintf("%d-%d", startNew, endNew)
		}
	}

	return CommentTask{
		Path:           currentPath,
		LineSpec:       lineSpec,
		Side:           side,
		Body:           body,
		ReviewFileLine: lineNum,
	}, ""
}

// truncateFirstLine extracts the first line of the body and truncates it to maxLen runes.
func truncateFirstLine(s string, maxLen int) string {
	if idx := strings.IndexByte(s, '\n'); idx != -1 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return string(runes)
}

func runConcurrently(reviewFilePath string, prDetails PRDetails, prNumber, commitSHA string, comments []CommentTask) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	results := make([]commentResult, len(comments))

	endpoint := fmt.Sprintf("repos/%s/pulls/%s/comments", prDetails.BaseRepo, prNumber)

	for i, c := range comments {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, task CommentTask) {
			defer wg.Done()
			defer func() { <-sem }()

			args := []string{
				"api",
				"--hostname", prDetails.Host,
				endpoint,
				"-f", fmt.Sprintf("body=%s", task.Body),
				"-f", fmt.Sprintf("commit_id=%s", commitSHA),
				"-f", fmt.Sprintf("path=%s", task.Path),
				"-f", fmt.Sprintf("side=%s", task.Side),
			}

			if strings.Contains(task.LineSpec, "-") {
				parts := strings.Split(task.LineSpec, "-")
				args = append(args,
					"-F", fmt.Sprintf("start_line=%s", parts[0]),
					"-F", fmt.Sprintf("line=%s", parts[1]),
					"-f", fmt.Sprintf("start_side=%s", task.Side),
				)
			} else {
				args = append(args,
					"-F", fmt.Sprintf("line=%s", task.LineSpec),
				)
			}

			cmd := exec.Command("gh", args...)
			out, err := cmd.CombinedOutput()

			if err != nil {
				rawOutput := strings.TrimSpace(string(out))

				var hint string
				if strings.Contains(rawOutput, "404") {
					hint = fmt.Sprintf(" (Hint: HTTP 404 on host %s means commit %s or target path is missing)", prDetails.Host, commitSHA[:7])
				} else if strings.Contains(rawOutput, "422") {
					hint = " (Hint: HTTP 422 usually means the line/range is outside the PR diff context)"
				}

				results[idx] = commentResult{
					task: task,
					err: fmt.Errorf(
						"%s:%d: gh api comment failed%s\n"+
							"  Host       : %s\n"+
							"  Repository : %s\n"+
							"  PR Number  : %s\n"+
							"  Commit SHA : %s\n"+
							"  Target File: %s\n"+
							"  Line Spec  : %s (side: %s)\n"+
							"  Command    : gh %s\n"+
							"  Output     : %s",
						reviewFilePath, task.ReviewFileLine, hint,
						prDetails.Host, prDetails.BaseRepo, prNumber, commitSHA, task.Path, task.LineSpec, task.Side,
						strings.Join(args, " "), rawOutput,
					),
				}
			} else {
				results[idx] = commentResult{task: task}
			}
		}(i, c)
	}

	wg.Wait()

	hasErr := false
	for _, res := range results {
		if res.err != nil {
			fmt.Fprintln(os.Stderr, "Error:", res.err)
			hasErr = true
		} else {
			firstLine := truncateFirstLine(res.task.Body, 60)
			fmt.Printf("%s: %s\n", res.task.Path, firstLine)
		}
	}

	if hasErr {
		os.Exit(1)
	}
}