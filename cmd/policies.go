package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type listRawPolicyFile struct {
	Policies []listRawPolicy `yaml:"policies"`
}

type listRawPolicy struct {
	Name     string `yaml:"name"`
	Priority int    `yaml:"priority"`
	Enabled  *bool  `yaml:"enabled"`
	Match    struct {
		Roles    []string `yaml:"roles"`
		Resource string   `yaml:"resource"`
	} `yaml:"match"`
}

var policiesCmd = &cobra.Command{
	Use:   "policies",
	Short: "Inspect offline policy configurations",
}

var policiesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all defined policy rules in policies directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		dirPath := resolvePath(policiesDir)
		
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			printError("Policies directory not found: %s", dirPath)
			printDimmed("Run 'auvis init' to create it")
			return nil
		}

		var files []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".yaml" || ext == ".yml" {
				files = append(files, filepath.Join(dirPath, entry.Name()))
			}
		}

		if len(files) == 0 {
			printWarning("No policy files found in %s", policiesDir)
			return nil
		}

		var allPolicies []listRawPolicy
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				printWarning("Could not read policy file %s: %v", filepath.Base(f), err)
				continue
			}

			var pf listRawPolicyFile
			if err := yaml.Unmarshal(data, &pf); err != nil {
				printWarning("Invalid YAML in policy file %s: %v", filepath.Base(f), err)
				continue
			}

			allPolicies = append(allPolicies, pf.Policies...)
		}

		fmt.Println()
		bold.Println("  AUVIS Policy Inspector")
		separator()
		
		// Print table header
		fmt.Printf("  %-30s %-20s %-10s %-12s\n", "NAME", "RESOURCE", "PRIORITY", "STATUS")
		dimmed.Println("  ─────────────────────────────────────────────────────────────────────────────")

		for _, p := range allPolicies {
			statusStr := "🟢 Enabled"
			if p.Enabled != nil && !*p.Enabled {
				statusStr = "🔴 Disabled"
			}
			
			// Clean names/resources if they are too long for nice layout
			name := p.Name
			if len(name) > 28 {
				name = name[:25] + "..."
			}
			res := p.Match.Resource
			if len(res) > 18 {
				res = res[:15] + "..."
			}

			fmt.Printf("  %-30s %-20s %-10d %s\n", name, res, p.Priority, statusStr)
		}

		dimmed.Println("  ─────────────────────────────────────────────────────────────────────────────")
		fmt.Println()

		return nil
	},
}
