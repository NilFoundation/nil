package version

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/spf13/cobra"
)

const (
	appTitle = "=;Nil CLI"
)

func GetCommand() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:          "version",
		Short:        "Get the current version",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			PrintVersionString()
		},
	}
	return versionCmd
}

func PrintVersionString() {
	fmt.Println(version.BuildVersionString(appTitle))
}
