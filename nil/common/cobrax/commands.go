package cobrax

import (
	"os"

	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/spf13/cobra"
)

func ExitOnHelp(c *cobra.Command) {
	helpFunc := c.HelpFunc()
	c.SetHelpFunc(func(c *cobra.Command, s []string) {
		helpFunc(c, s)
		os.Exit(0)
	})
}

func VersionCmd(title string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(version.BuildVersionString(title))
			os.Exit(0)
		},
	}
}
