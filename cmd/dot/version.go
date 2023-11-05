package dot

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version   = "nightly"
	builddate = "unknown"
	commit    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Shows the current version of Done CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Version:", version)
		fmt.Println("Build Date:", builddate)
		fmt.Println("Commit:", commit)
	},
}
