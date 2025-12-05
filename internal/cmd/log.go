package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/otherjamesbrown/context-palace/internal/cxpdir"
	"github.com/otherjamesbrown/context-palace/internal/logging"
	"github.com/otherjamesbrown/context-palace/internal/output"
	"github.com/spf13/cobra"
)

var (
	logLimit int
	logType  string
	logJSON  bool
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show recent access log",
	Long: `Show recent memo access and write logs.

Examples:
  cxp log              # Show last 20 access entries
  cxp log -n 50        # Show last 50 entries
  cxp log --type writes # Show write log
  cxp log --json       # Output as JSON`,
	RunE: runLog,
}

func init() {
	logCmd.Flags().IntVarP(&logLimit, "limit", "n", 20, "Number of entries to show")
	logCmd.Flags().StringVar(&logType, "type", "access", "Log type: 'access' or 'writes'")
	logCmd.Flags().BoolVar(&logJSON, "json", false, "Output as JSON")
}

type LogOutput struct {
	Type    string        `json:"type"`
	Entries []interface{} `json:"entries"`
}

func runLog(cmd *cobra.Command, args []string) error {
	// Find .cxp root
	root, err := cxpdir.FindRoot()
	if err != nil {
		if errors.Is(err, cxpdir.ErrNotInitialized) {
			output.PrintError(".cxp not initialized. Run 'cxp init' first.")
		}
		return err
	}

	cfg, err := cxpdir.LoadConfig(root)
	if err != nil {
		output.PrintError("Could not load config: %v", err)
		return err
	}

	// Determine log file
	var logFile string
	switch logType {
	case "access":
		logFile = cfg.Logging.AccessLog
	case "writes":
		logFile = cfg.Logging.WritesLog
	default:
		output.PrintError("Invalid log type '%s'. Use 'access' or 'writes'", logType)
		return fmt.Errorf("invalid log type")
	}

	logPath := filepath.Join(root, cxpdir.DirName, logFile)

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		if logJSON {
			return output.Print(LogOutput{Type: logType, Entries: []interface{}{}}, true)
		}
		fmt.Println("No log entries yet")
		return nil
	}

	// Read log entries (JSONL format)
	entries, err := readLogEntries(logPath, logLimit)
	if err != nil {
		output.PrintError("Could not read log: %v", err)
		return err
	}

	if logJSON {
		return output.Print(LogOutput{Type: logType, Entries: entries}, true)
	}

	// Human-readable output
	if len(entries) == 0 {
		fmt.Println("No log entries yet")
		return nil
	}

	for _, entry := range entries {
		printLogEntry(entry, logType)
	}

	return nil
}

func readLogEntries(path string, limit int) ([]interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var allEntries []interface{}
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		allEntries = append(allEntries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Return last N entries
	if len(allEntries) <= limit {
		return allEntries, nil
	}
	return allEntries[len(allEntries)-limit:], nil
}

func printLogEntry(entry interface{}, logType string) {
	m, ok := entry.(map[string]interface{})
	if !ok {
		return
	}

	// Parse timestamp
	tsStr, _ := m["ts"].(string)
	ts, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		ts, _ = time.Parse(time.RFC3339, tsStr)
	}

	timeStr := ts.Format("2006-01-02 15:04:05")

	if logType == "access" {
		memo, _ := m["memo"].(string)
		depth, _ := m["depth"].(string)
		if depth == "" || depth == "0" {
			fmt.Printf("%s  %s\n", timeStr, memo)
		} else {
			fmt.Printf("%s  %s (depth: %s)\n", timeStr, memo, depth)
		}
	} else {
		// writes log
		op, _ := m["op"].(string)
		memo, _ := m["memo"].(string)
		parent, _ := m["parent"].(string)

		if parent != "" {
			fmt.Printf("%s  [%s] %s (parent: %s)\n", timeStr, op, memo, parent)
		} else {
			fmt.Printf("%s  [%s] %s\n", timeStr, op, memo)
		}
	}
}

// Helper to parse access log entry
func parseAccessEntry(data []byte) (*logging.AccessEntry, error) {
	var entry logging.AccessEntry
	err := json.Unmarshal(data, &entry)
	return &entry, err
}

// Helper to parse write log entry
func parseWriteEntry(data []byte) (*logging.WriteEntry, error) {
	var entry logging.WriteEntry
	err := json.Unmarshal(data, &entry)
	return &entry, err
}
