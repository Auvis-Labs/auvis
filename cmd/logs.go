package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cridiv/Auvis/audit"
	"github.com/cridiv/Auvis/config"
)

var (
	logRole     string
	logUser     string
	logResource string
	logLimit    int
	logFollow   bool
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Tail and filter the audit log",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadConfig(cfgFile)
		auditPath := resolvePath(cfg.AuditFile)

		// Check if file exists
		if _, err := os.Stat(auditPath); os.IsNotExist(err) {
			if !logFollow {
				printWarning("No audit logs found yet (audit file %s does not exist)", cfg.AuditFile)
				return nil
			}
			printInfo("Waiting for audit log file to be created: %s ...", cfg.AuditFile)
			for {
				if _, err := os.Stat(auditPath); err == nil {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
		}

		// 1. Parse and filter history
		data, err := os.ReadFile(auditPath)
		if err != nil {
			printError("Could not read audit log file: %v", err)
			return nil
		}

		allEntries := parseAuditEntries(data)
		var filtered []audit.Entry
		for _, e := range allEntries {
			if matchLogFilters(e) {
				filtered = append(filtered, e)
			}
		}

		// Apply limit if specified
		startIdx := 0
		if logLimit > 0 && len(filtered) > logLimit {
			startIdx = len(filtered) - logLimit
		}
		
		fmt.Println()
		bold.Printf("  AUVIS Audit Stream")
		if logRole != "" || logUser != "" || logResource != "" {
			var filters []string
			if logRole != "" {
				filters = append(filters, "role="+logRole)
			}
			if logUser != "" {
				filters = append(filters, "user="+logUser)
			}
			if logResource != "" {
				filters = append(filters, "resource="+logResource)
			}
			dimmed.Printf(" (Filters: %s)", strings.Join(filters, ", "))
		}
		fmt.Println()
		separator()

		for i := startIdx; i < len(filtered); i++ {
			fmt.Println(formatLogEntry(filtered[i]))
		}

		// 2. Tailing/Following
		if logFollow {
			f, err := os.Open(auditPath)
			if err != nil {
				printError("Could not open audit log for tailing: %v", err)
				return nil
			}
			defer f.Close()

			// Seek to the end so we only get new appends
			_, err = f.Seek(0, io.SeekEnd)
			if err != nil {
				printError("Could not seek audit log: %v", err)
				return nil
			}

			reader := bufio.NewReader(f)
			var block bytes.Buffer
			for {
				line, err := reader.ReadString('\n')
				if err == io.EOF {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				if err != nil {
					printError("Error reading audit stream: %v", err)
					return nil
				}

				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					if block.Len() > 0 {
						var e audit.Entry
						if err := json.Unmarshal(block.Bytes(), &e); err == nil {
							if matchLogFilters(e) {
								fmt.Println(formatLogEntry(e))
							}
						}
						block.Reset()
					}
					continue
				}
				block.WriteString(line)
			}
		}

		separator()
		fmt.Println()
		return nil
	},
}

func init() {
	logsCmd.Flags().StringVar(&logRole, "role", "", "Filter logs by role")
	logsCmd.Flags().StringVar(&logUser, "user", "", "Filter logs by user")
	logsCmd.Flags().StringVar(&logResource, "resource", "", "Filter logs by resource path")
	logsCmd.Flags().IntVar(&logLimit, "limit", 20, "Number of past log entries to show")
	logsCmd.Flags().BoolVarP(&logFollow, "follow", "f", false, "Follow/tail the audit logs in real-time")
}

func parseAuditEntries(data []byte) []audit.Entry {
	var entries []audit.Entry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var block bytes.Buffer

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if block.Len() > 0 {
				var e audit.Entry
				if err := json.Unmarshal(block.Bytes(), &e); err == nil {
					entries = append(entries, e)
				}
				block.Reset()
			}
			continue
		}
		block.WriteString(line)
		block.WriteByte('\n')
	}

	if block.Len() > 0 {
		var e audit.Entry
		if err := json.Unmarshal(block.Bytes(), &e); err == nil {
			entries = append(entries, e)
		}
	}

	return entries
}

func matchLogFilters(e audit.Entry) bool {
	if logRole != "" && !strings.EqualFold(e.UserRole, logRole) {
		return false
	}
	if logUser != "" && !strings.EqualFold(e.UserID, logUser) {
		return false
	}
	if logResource != "" && !strings.HasPrefix(e.Resource, logResource) {
		return false
	}
	return true
}

func formatLogEntry(e audit.Entry) string {
	timestampStr := e.Timestamp.Local().Format("15:04:05")
	
	statusStr := fmt.Sprintf("%d", e.UpstreamStatus)
	if e.UpstreamStatus >= 200 && e.UpstreamStatus < 300 {
		statusStr = green.Sprint(statusStr)
	} else if e.UpstreamStatus >= 300 && e.UpstreamStatus < 400 {
		statusStr = cyan.Sprint(statusStr)
	} else if e.UpstreamStatus >= 400 && e.UpstreamStatus < 500 {
		statusStr = yellow.Sprint(statusStr)
	} else if e.UpstreamStatus >= 500 {
		statusStr = red.Sprint(statusStr)
	}

	methodStr := bold.Sprint(e.Method)
	resStr := e.Resource
	userStr := cyan.Sprint(e.UserID)
	roleStr := dimmed.Sprintf("[%s]", e.UserRole)
	durStr := dimmed.Sprintf("(%dms)", e.DurationMS)

	maskedStr := ""
	if len(e.FieldsMasked) > 0 {
		maskedStr = yellow.Sprintf(" [Masked: %s]", strings.Join(e.FieldsMasked, ", "))
	}

	policiesStr := ""
	if len(e.PoliciesApplied) > 0 {
		policiesStr = dimmed.Sprintf(" [Policies: %s]", strings.Join(e.PoliciesApplied, ", "))
	}

	return fmt.Sprintf("  %s  %s %-16s %s %s -> %s %s%s%s", 
		timestampStr, methodStr, resStr, userStr, roleStr, statusStr, durStr, maskedStr, policiesStr)
}
