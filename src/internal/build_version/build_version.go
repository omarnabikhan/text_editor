package build_version

type BuildVersion string

// If the binary is built with the "dev" tag, it'll be in DEV mode. Otherwise, it's PROD.
// Note: the binary is not explicitly built with a "prod" tag, just the absense of "dev" implies this.
var (
	PROD BuildVersion = "PROD"
	DEV  BuildVersion = "DEV"
)
