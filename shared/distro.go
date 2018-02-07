package shared

// GetArch returns the correct architecture name used by the specified
// distribution.
func GetArch(distro, arch string) string {
	switch distro {
	case "alpinelinux", "archlinux", "centos":
		if arch == "amd64" {
			return "x86_64"
		}
	case "debian", "ubuntu":
		if arch == "x86_64" {
			return "amd64"
		}
	}

	return arch
}
