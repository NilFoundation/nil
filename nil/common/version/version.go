package version

import (
	"errors"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/NilFoundation/nil/nil/common"
)

type versionInfo struct {
	GitTag      string
	GitRevision string
	GitCommit   string
}

var (
	// This random string will be binary-patched to contain real version.
	// This is done in order to not stamp the commit hash on build and
	// thus not needlessly thrash the cache. Instead, this string will
	// be replaced on package-build time.
	versionMagic          string = "qm5h7IEa3ahXUgsPknK8bwWulPEmpgMWSaQSaOUa"
	versionInfoCache      versionInfo
	versionInfoCacheMutex sync.Mutex
)

const (
	unknownRevision string = "0"
	unknownVersion  string = "<unknown>"
)

func GetVersionInfo() versionInfo {
	versionInfoCacheMutex.Lock()
	defer versionInfoCacheMutex.Unlock()
	if versionInfoCache.GitRevision == "" {
		// If the versionMagic string has been patched with something that looks like a version
		// then use it. Otherwise initialize the version with some sane default.
		re := regexp.MustCompile(`(\d+\.\d+\.\d+)-(\d+)-([a-f0-9]+)`)
		matches := re.FindStringSubmatch(versionMagic)

		if len(matches) == 0 {
			if _, gitCommit, err := ParseBuildInfo(); err == nil {
				versionInfoCache = versionInfo{GitTag: "0.1.0", GitRevision: "1", GitCommit: gitCommit}
			} else {
				versionInfoCache = versionInfo{GitTag: "0.1.0", GitRevision: "1", GitCommit: unknownVersion}
			}
		} else {
			versionInfoCache = versionInfo{GitTag: matches[1], GitRevision: matches[2], GitCommit: matches[3]}
		}
	}
	return versionInfoCache
}

func ParseBuildInfo() (string, string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", "", errors.New("failed to read build info")
	}
	var gitHash string
	var time string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			gitHash = s.Value
		case "vcs.time":
			time = s.Value[:10]
		}
	}

	return time, gitHash, nil
}

func HasGitInfo() bool {
	return GetVersionInfo().GitCommit != unknownVersion
}

func BuildVersionString(appTitle string) string {
	ver := GetVersionInfo().GitTag
	if ver == "" {
		ver = unknownVersion
	}

	parts := strings.Split(ver, "-")

	// Tag can be in default format (e.g. "0.1.0") or
	// in a prefixed format (e.g. "nil_cli-2024.07.04").
	// For the second case first part should be skipped.
	ver = parts[0]
	if strings.HasPrefix(ver, "nil") {
		ver = parts[1]
	}

	return FormatVersion(versionTmpl, map[string]any{
		"Title":    appTitle,
		"Version":  ver,
		"OS":       runtime.GOOS,
		"Arch":     runtime.GOARCH,
		"Commit":   GetVersionInfo().GitCommit,
		"Revision": GetGitRevision(),
	})
}

func GetGitRevision() string {
	if GetVersionInfo().GitRevision == "" {
		return unknownRevision
	}
	return GetVersionInfo().GitRevision
}

func FormatVersion(template string, templateArgs map[string]any) string {
	versionMsg, err := common.ParseTemplate(template, templateArgs)
	if err != nil {
		panic(err)
	}

	return versionMsg
}

var versionTmpl = `{{ .Title }}
 Version:	{{ .Version }}
 OS/Arch:	{{ .OS }}/{{ .Arch }}
 Git commit:	{{ .Commit }}
 Revision:	{{ .Revision }}`
