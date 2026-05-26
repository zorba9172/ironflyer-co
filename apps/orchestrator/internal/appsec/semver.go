package appsec

import (
	"strconv"
	"strings"
)

func normaliseVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimLeft(version, "=^~<> ")
	if idx := strings.IndexAny(version, " +"); idx >= 0 {
		version = version[:idx]
	}
	return version
}

func semverLess(a, b string) bool {
	aa := parseSemver(a)
	bb := parseSemver(b)
	for i := 0; i < 3; i++ {
		if aa[i] < bb[i] {
			return true
		}
		if aa[i] > bb[i] {
			return false
		}
	}
	return false
}

func versionInList(version string, values ...string) bool {
	version = normaliseVersion(version)
	for _, v := range values {
		if version == normaliseVersion(v) {
			return true
		}
	}
	return false
}

func versionInRange(version, min, max string) bool {
	version = normaliseVersion(version)
	return !semverLess(version, min) && !semverLess(max, version)
}

func parseSemver(version string) [3]int {
	version = normaliseVersion(version)
	var out [3]int
	parts := strings.Split(version, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		part := parts[i]
		if idx := strings.IndexByte(part, '-'); idx >= 0 {
			part = part[:idx]
		}
		n, _ := strconv.Atoi(part)
		out[i] = n
	}
	return out
}
