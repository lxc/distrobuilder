package shared

import (
	"fmt"

	"github.com/lxc/lxd/shared/osarch"
)

var alpineLinuxArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86:             "x86",
	osarch.ARCH_64BIT_INTEL_X86:             "x86_64",
	osarch.ARCH_32BIT_ARMV7_LITTLE_ENDIAN:   "armhf",
	osarch.ARCH_64BIT_ARMV8_LITTLE_ENDIAN:   "aarch64",
	osarch.ARCH_64BIT_POWERPC_LITTLE_ENDIAN: "ppc64le",
}

var archLinuxArchitectureNames = map[int]string{
	osarch.ARCH_64BIT_INTEL_X86: "x86_64",
}

var centosArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86: "i386",
	osarch.ARCH_64BIT_INTEL_X86: "x86_64",
}

var debianArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86:             "i386",
	osarch.ARCH_64BIT_INTEL_X86:             "amd64",
	osarch.ARCH_32BIT_ARMV7_LITTLE_ENDIAN:   "armhf",
	osarch.ARCH_64BIT_ARMV8_LITTLE_ENDIAN:   "arm64",
	osarch.ARCH_32BIT_POWERPC_BIG_ENDIAN:    "powerpc",
	osarch.ARCH_64BIT_POWERPC_BIG_ENDIAN:    "powerpc64",
	osarch.ARCH_64BIT_POWERPC_LITTLE_ENDIAN: "ppc64le",
}

var distroArchitecture = map[string]map[int]string{
	"alpinelinux": alpineLinuxArchitectureNames,
	"archlinux":   archLinuxArchitectureNames,
	"centos":      centosArchitectureNames,
	"debian":      debianArchitectureNames,
	"ubuntu":      debianArchitectureNames,
}

// GetArch returns the correct architecture name used by the specified
// distribution.
func GetArch(distro, arch string) (string, error) {
	archMap, ok := distroArchitecture[distro]
	if !ok {
		return "unknown", fmt.Errorf("Architecture map isn't supported: %s", distro)
	}

	archID, err := osarch.ArchitectureId(arch)
	if err != nil {
		return "unknown", err
	}

	archName, exists := archMap[archID]
	if exists {
		return archName, nil
	}

	return "unknown", fmt.Errorf("Architecture isn't supported: %s", arch)
}
