package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/miltonparedes/kitmux/internal/views/palette"
)

func addCommandsCommand(parent *cobra.Command) {
	parent.AddCommand(&cobra.Command{
		Use:   "commands",
		Short: "List all available command IDs",
		Run: func(_ *cobra.Command, _ []string) {
			var lastCat string
			for _, c := range palette.DefaultCommands() {
				if c.Category != lastCat {
					if lastCat != "" {
						fmt.Println()
					}
					fmt.Printf("  %s:\n", c.Category)
					lastCat = c.Category
				}
				fmt.Printf("    %-24s %s\n", c.ID, c.Description)
			}
		},
	})
}
