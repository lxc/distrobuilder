package shared

import (
	"fmt"

	"github.com/lxc/lxd/shared/osarch"
)

var alpineLinuxArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86:           "x86",
	osarch.ARCH_32BIT_ARMV7_LITTLE_ENDIAN: "armhf",
}

var archLinuxArchitectureNames = map[int]string{
	osarch.ARCH_64BIT_INTEL_X86:           "x86_64",
	osarch.ARCH_32BIT_ARMV7_LITTLE_ENDIAN: "armv7",
	osarch.ARCH_64BIT_ARMV8_LITTLE_ENDIAN: "aarch64",
}

var centosArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86: "i386",
}

var debianArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86:             "i386",
	osarch.ARCH_64BIT_INTEL_X86:             "amd64",
	osarch.ARCH_32BIT_ARMV7_LITTLE_ENDIAN:   "armhf",
	osarch.ARCH_64BIT_ARMV8_LITTLE_ENDIAN:   "arm64",
	osarch.ARCH_32BIT_POWERPC_BIG_ENDIAN:    "powerpc",
	osarch.ARCH_64BIT_POWERPC_BIG_ENDIAN:    "powerpc64",
	osarch.ARCH_64BIT_POWERPC_LITTLE_ENDIAN: "ppc64el",
}

var gentooArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86:             "i686",
	osarch.ARCH_64BIT_INTEL_X86:             "amd64",
	osarch.ARCH_32BIT_ARMV7_LITTLE_ENDIAN:   "armv7a_hardfp",
	osarch.ARCH_32BIT_POWERPC_BIG_ENDIAN:    "ppc",
	osarch.ARCH_64BIT_POWERPC_BIG_ENDIAN:    "ppc64",
	osarch.ARCH_64BIT_POWERPC_LITTLE_ENDIAN: "ppc64le",
	osarch.ARCH_64BIT_S390_BIG_ENDIAN:       "s390x",
}

var plamoLinuxArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86: "x86",
}

var altLinuxArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86:           "i586",
	osarch.ARCH_64BIT_INTEL_X86:           "x86_64",
	osarch.ARCH_64BIT_ARMV8_LITTLE_ENDIAN: "aarch64",
}

var voidLinuxArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86:           "i686",
	osarch.ARCH_64BIT_INTEL_X86:           "x86_64",
	osarch.ARCH_32BIT_ARMV7_LITTLE_ENDIAN: "armv7l",
	osarch.ARCH_64BIT_ARMV8_LITTLE_ENDIAN: "aarch64",
}

var funtooArchitectureNames = map[int]string{
	osarch.ARCH_32BIT_INTEL_X86:           "generic_32",
	osarch.ARCH_64BIT_INTEL_X86:           "generic_64",
	osarch.ARCH_32BIT_ARMV7_LITTLE_ENDIAN: "armv7a_vfpv3_hardfp",
	osarch.ARCH_64BIT_ARMV8_LITTLE_ENDIAN: "arm64_generic",
}

var distroArchitecture = map[string]map[int]string{
	"alpinelinux": alpineLinuxArchitectureNames,
	"altlinux":    altLinuxArchitectureNames,
	"archlinux":   archLinuxArchitectureNames,
	"centos":      centosArchitectureNames,
	"debian":      debianArchitectureNames,
	"gentoo":      gentooArchitectureNames,
	"plamolinux":  plamoLinuxArchitectureNames,
	"voidlinux":   voidLinuxArchitectureNames,
	"funtoo":      funtooArchitectureNames,
}

// GetArch returns the correct architecture name used by the specified
// distribution.
func GetArch(distro, arch string) (string, error) {
	// Special case armel as it is effectively a different userspace variant
	// of armv7 without hard-float and so doesn't have its own kernel architecture name
	if arch == "armel" {
		return "armel", nil
	}

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

	return arch, nil
}
