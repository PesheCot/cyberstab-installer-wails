package installer

type LinuxOSInfo struct {
	Type    string
	Edition string
	Version string
}

type PostgresPackageOption struct {
	Version     string
	Label       string
	PackageName string
}
