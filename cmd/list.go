package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/benly/k10s/internal/config"
	"github.com/benly/k10s/internal/profile"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all kubeconfig profiles",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	profiles, err := profile.Scan(cfg)
	if err != nil {
		return fmt.Errorf("scanning profiles: %w", err)
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles found.")
		fmt.Printf("Configs directory: %s\n", config.ExpandPath(cfg.Global.ConfigsDir))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSERVER\tACTION\tOIDC")
	fmt.Fprintln(w, "----\t------\t------\t----")

	for _, p := range profiles {
		oidc := ""
		if p.OIDC {
			oidc = "true"
		}
		server := p.ServerURL
		if server == "" {
			server = "(unknown)"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, server, p.DefaultAction, oidc)
	}

	return w.Flush()
}
