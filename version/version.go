package version

import "fmt"

const (
	majorVersion uint32 = 0
	minorVersion uint32 = 0
	patchVersion uint32 = 1
)

var (
	gitCommit     string
	ver           *Version
	compatibleVer *Version
)

type Version struct {
	majorVersion  uint32
	minorVersion  uint32
	patchVersion  uint32
	versionString string
}

// Format version to "<majorVersion>.<minorVersion>.<patchVersion>[+<gitCommit>]",
// like "1.0.0", or "1.0.0+1a2b3c4d".
func (v Version) String() string {
	if v.versionString != "" {
		return v.versionString
	}

	v.versionString = fmt.Sprintf("%d.%d.%d", v.majorVersion, v.minorVersion, v.patchVersion)
	if gitCommit != "" && len(gitCommit) >= 8 {
		v.versionString += "+" + gitCommit[:8]
	}
	return v.versionString
}

func GetVersion() *Version {
	return ver
}

func GetCompatibleVersion() *Version {
	return compatibleVer
}

func init() {
	ver = &Version{
		majorVersion: majorVersion,
		minorVersion: minorVersion,
		patchVersion: patchVersion,
	}
	compatibleVer = ver
}
