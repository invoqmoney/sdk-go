package invoq

import (
	"runtime/debug"
	"strings"
)

const (
	modulePath     = "github.com/invoqmoney/sdk-go"
	unknownVersion = "unknown"
	userAgentName  = "invoq-go"
)

var sdkVersion = readSDKVersion()

func userAgent() string {
	return userAgentName + "/" + sdkVersion
}

func readSDKVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return unknownVersion
	}

	return sdkVersionFromBuildInfo(buildInfo)
}

func sdkVersionFromBuildInfo(buildInfo *debug.BuildInfo) string {
	if buildInfo == nil {
		return unknownVersion
	}

	if buildInfo.Main.Path == modulePath {
		return moduleVersion(buildInfo.Main)
	}

	for _, dependency := range buildInfo.Deps {
		if dependency.Path == modulePath {
			return moduleVersion(*dependency)
		}
	}

	return unknownVersion
}

func moduleVersion(module debug.Module) string {
	if module.Replace != nil {
		if module.Replace.Version == "" {
			return unknownVersion
		}

		return normalizeModuleVersion(module.Replace.Version)
	}

	return normalizeModuleVersion(module.Version)
}

func normalizeModuleVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "(devel)" {
		return unknownVersion
	}

	if hasGoVersionPrefix(version) {
		return version[1:]
	}

	return version
}

func hasGoVersionPrefix(version string) bool {
	return len(version) > 1 &&
		(version[0] == 'v' || version[0] == 'V') &&
		isASCIIDigit(version[1])
}

func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}
