package version

import (
	"bytes"
	"runtime"
	"strings"
	"text/template"

	"github.com/NilFoundation/nil/nil/common/check"
)

var (
	gitTag      string
	gitCommit   string
	gitRevision string
)

const (
	unknownRevision string = "0"
	unknownVersion  string = "<unknown>"
)

func BuildVersionString(appTitle string) string {
	ver := gitTag
	if ver == "" {
		ver = unknownVersion
	}

	parts := strings.SplitN(ver, "-", 2)
	check.PanicIfNot(len(parts) > 0)
	ver = parts[0]

	return formatVersion(versionTmpl, map[string]string{
		"Title":    appTitle,
		"Version":  ver,
		"OS":       runtime.GOOS,
		"Arch":     runtime.GOARCH,
		"Commit":   gitCommit,
		"Revision": GetGitRevision(),
	})
}

func GetGitRevision() string {
	if gitRevision == "" {
		return unknownRevision
	}
	return gitRevision
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

var versionTmpl = `{{ .Title }}
 Version:	{{ .Version }}
 OS/Arch: 	{{ .OS }}/{{ .Arch }}
 Git commit:	{{ .Commit }}
 Revision:	{{ .Revision }}`
