package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/benly/k10s/internal/deps"
	"github.com/spf13/cobra"
)

var (
	fixDeps    bool
	fixAllDeps bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check and optionally install required dependencies",
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().BoolVar(&fixDeps, "fix", false, "Automatically install missing required dependencies via brew")
	doctorCmd.Flags().BoolVar(&fixAllDeps, "fix-all", false, "Automatically install all missing dependencies (including optional) via brew")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	result := deps.Check()
	deps.PrintReport(result)

	if result.OK && !fixAllDeps {
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	for _, d := range result.Deps {
		if d.Found {
			continue
		}

		// Skip optional deps unless --fix-all is true
		if !d.Required && !fixAllDeps {
			continue
		}

		// If --fix or --fix-all is true, install automatically without prompting
		autoInstall := fixDeps && d.Required || fixAllDeps
		
		brewPkg := d.Brew
		if brewPkg == "" {
			brewPkg = d.Name
		}

		if !autoInstall {
			fmt.Printf("Install %s via brew? [y/N] ", brewPkg)
			answer, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				continue
			}
		} else {
			fmt.Printf("Auto-fixing missing dependency: %s\n", brewPkg)
		}

		if err := deps.InstallViaBrew(brewPkg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}

	return nil
}
