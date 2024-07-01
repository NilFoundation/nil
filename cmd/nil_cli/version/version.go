package version

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"text/template"

	"github.com/NilFoundation/nil/common/check"
	"github.com/spf13/cobra"
)

var (
	gitTag    string
	gitCommit string
)

const (
	unknownVersion = "<unknown>"
	versionTitle   = "=;Nil CLI"
)

func GetCommand() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:          "version",
		Short:        "Get current version",
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			PrintVersionString()
		},
	}
	return versionCmd
}

func buildVersionString() string {
	if gitTag == "" {
		gitTag = unknownVersion
	}

	parts := strings.SplitN(gitTag, "-", 2)
	check.PanicIfNot(len(parts) > 0)
	gitTag = parts[0]

	return formatVersion(cliVersionTmpl, map[string]string{
		"Title":   versionTitle,
		"Version": gitTag,
		"OS":      runtime.GOOS,
		"Arch":    runtime.GOARCH,
		"Commit":  gitCommit,
	})
}

func PrintVersionString() {
	fmt.Println(buildVersionString())
}

func getTemplatedStr(text *string, obj interface{}) (string, error) {
	tmpl, err := template.New("s").Parse(*text)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if err = tmpl.Execute(buf, obj); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func formatVersion(template string, templateArgs map[string]string) string {
	versionMsg, err := getTemplatedStr(&template, templateArgs)
	if err != nil {
		panic(err)
	}

	return versionMsg
}

var cliVersionTmpl = `{{ .Title }}
 Version:	{{ .Version }}
 OS/Arch: 	{{ .OS }}/{{ .Arch }}
 Git commit:	{{ .Commit }}`
