package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Auvis-Labs/auvis/config"
)

type policyStatusInfo struct {
	Role           string   `json:"role"`
	Resource       string   `json:"resource"`
	SourcePolicies []string `json:"source_policies"`
	Rules          []string `json:"rules"`
}

type statusResponse struct {
	Status         string             `json:"status"`
	Uptime         string             `json:"uptime"`
	StartedAt      string             `json:"started_at"`
	RequestsTotal  int64              `json:"requests_total"`
	ActivePolicies []policyStatusInfo `json:"active_policies"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show gateway health and active policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadConfig(cfgFile)

		client := http.Client{
			Timeout: 2 * time.Second,
		}

		resp, err := client.Get(fmt.Sprintf("http://localhost:%s/auvis/status", cfg.Port))
		if err != nil {
			printError("Gateway is not running (could not connect to localhost:%s)", cfg.Port)
			printDimmed("Start the gateway first with: auvis dev")
			return nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			printError("Gateway status endpoint returned HTTP %d", resp.StatusCode)
			return nil
		}

		var status statusResponse
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			printError("Failed to decode gateway status response: %v", err)
			return nil
		}

		fmt.Println()
		bold.Println("  AUVIS Gateway Status")
		separator()

		fmt.Printf("  %-17s ", "Status:")
		green.Println("Online (OK)")

		fmt.Printf("  %-17s %s\n", "Uptime:", status.Uptime)
		fmt.Printf("  %-17s %d\n", "Total Requests:", status.RequestsTotal)
		fmt.Printf("  %-17s %d effective rules active\n", "Active Policies:", len(status.ActivePolicies))

		if len(status.ActivePolicies) > 0 {
			fmt.Println()
			bold.Println("  Active Rules:")
			for _, ap := range status.ActivePolicies {
				rulesStr := strings.Join(ap.Rules, ", ")
				sourcesStr := strings.Join(ap.SourcePolicies, ", ")
				
				fmt.Printf("    ")
				green.Print("✓ ")
				bold.Printf("[%s] ", ap.Role)
				fmt.Printf("%s", ap.Resource)
				dimmed.Print(" -> ")
				fmt.Printf("%s ", rulesStr)
				printDimmed("(from %s)", sourcesStr)
			}
		}

		separator()
		fmt.Println()

		return nil
	},
}
