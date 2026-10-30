package check

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var numberPattern = regexp.MustCompile(`\d+`)

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "go")
	return version
}

func IsNewer(latest, installed string) bool {
	latest = normalizeVersion(latest)
	installed = normalizeVersion(installed)
	if latest == "" || installed == "" {
		return false
	}
	if strings.EqualFold(latest, "unknown") || strings.EqualFold(installed, "unknown") {
		return false
	}

	latestVersion, latestErr := semver.NewVersion(latest)
	installedVersion, installedErr := semver.NewVersion(installed)
	if latestErr == nil && installedErr == nil {
		return latestVersion.GreaterThan(installedVersion)
	}
	return compareNumbers(latest, installed) > 0
}

func compareNumbers(a, b string) int {
	left := numberPattern.FindAllString(a, -1)
	right := numberPattern.FindAllString(b, -1)
	length := len(left)
	if len(right) > length {
		length = len(right)
	}
	for i := 0; i < length; i++ {
		var lv, rv int
		if i < len(left) {
			lv, _ = strconv.Atoi(left[i])
		}
		if i < len(right) {
			rv, _ = strconv.Atoi(right[i])
		}
		if lv < rv {
			return -1
		}
		if lv > rv {
			return 1
		}
	}
	return 0
}
