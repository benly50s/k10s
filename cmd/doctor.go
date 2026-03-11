package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/benly/k10s/internal/deps"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check and optionally install required dependencies",
	RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	result := deps.Check()
	deps.PrintReport(result)

	if result.OK {
		return nil
	}

	// Offer to install missing deps via brew
	reader := bufio.NewReader(os.Stdin)
	for _, d := range result.Deps {
		if d.Found || !d.Required {
			continue
		}

		fmt.Printf("Install %s via brew? [y/N] ", d.Name)
		answer, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "y" || answer == "yes" {
			brewPkg := d.Brew
			if brewPkg == "" {
				brewPkg = d.Name
			}
			if err := deps.InstallViaBrew(brewPkg); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		}
	}

	return nil
}
