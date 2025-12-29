//go:build !dev

package build_version

func GetVersion() BuildVersion {
	return PROD
}
