package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v4"
	incus "github.com/lxc/incus/v6/shared/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/windows"
)

type cmdRepackWindows struct {
	global *cmdGlobal

	flagDrivers             string
	flagWindowsVersion      string
	flagWindowsArchitecture string

	defaultDrivers string
	umounts        []string
}

func init() {
	// Filters should be registered in the init() function
	_ = pongo2.RegisterFilter("toHex", toHex)
}

func (c *cmdRepackWindows) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repack-windows <source-iso> <target-iso> [--drivers=DRIVERS]",
		Short:   "Repack Windows ISO with drivers included",
		Args:    cobra.ExactArgs(2),
		PreRunE: c.preRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := c.global.logger
			defer func() {
				for i := len(c.umounts) - 1; i >= 0; i-- {
					dir := c.umounts[i]
					logger.Infof("Umount dir %q", dir)
					_ = unix.Unmount(dir, 0)
				}
			}()

			sourceDir := filepath.Dir(args[0])
			targetDir := filepath.Dir(args[1])

			// If either the source or the target are located on a FUSE filesystem, disable overlay
			// as it doesn't play well with wimlib-imagex.
			for _, dir := range []string{sourceDir, targetDir} {
				var stat unix.Statfs_t

				err := unix.Statfs(dir, &stat)
				if err != nil {
					logger.WithFields(logrus.Fields{"dir": dir, "err": err}).Warn("Failed to get directory information")
					continue
				}

				// Since there's no magic number for virtiofs, we need to check FUSE_SUPER_MAGIC (which is not defined in the unix package).
				if stat.Type == 0x65735546 {
					logger.Warn("FUSE filesystem detected, disabling overlay")
					c.global.flagDisableOverlay = true
					break
				}
			}

			overlayDir, cleanup, err := c.global.getOverlayDir()
			if err != nil {
				return fmt.Errorf("Failed to get overlay directory: %w", err)
			}

			if cleanup != nil {
				c.global.overlayCleanup = cleanup

				defer func() {
					cleanup()
					c.global.overlayCleanup = nil
				}()
			}

			return c.run(cmd, args, overlayDir)
		},
	}

	c.defaultDrivers = "virtio-win.iso"
	cmd.Flags().StringVar(&c.flagDrivers, "drivers", c.defaultDrivers, "Path to virtio windowns drivers ISO file"+"``")
	cmd.Flags().StringVar(&c.flagWindowsVersion, "windows-version", "",
		"Windows version to repack, must be one of ["+strings.Join(windows.SupportedWindowsVersions, ", ")+"]``")
	cmd.Flags().StringVar(&c.flagWindowsArchitecture, "windows-arch", "",
		"Windows architecture to repack, must be one of ["+strings.Join(windows.SupportedWindowsArchitectures, ", ")+"]``")

	return cmd
}

// Create rw rootfs in preRun. Point global.sourceDir to the rw rootfs.
func (c *cmdRepackWindows) preRun(cmd *cobra.Command, args []string) error {
	logger := c.global.logger

	if c.flagWindowsVersion == "" {
		c.flagWindowsVersion = windows.DetectWindowsVersion(filepath.Base(args[0]))
	} else {
		if !slices.Contains(windows.SupportedWindowsVersions, c.flagWindowsVersion) {
			return fmt.Errorf("Version must be one of %v", windows.SupportedWindowsVersions)
		}
	}

	if c.flagWindowsArchitecture == "" {
		c.flagWindowsArchitecture = windows.DetectWindowsArchitecture(filepath.Base(args[0]))
	} else {
		if !slices.Contains(windows.SupportedWindowsArchitectures, c.flagWindowsArchitecture) {
			return fmt.Errorf("Architecture must be one of %v", windows.SupportedWindowsArchitectures)
		}
	}

	// Check dependencies
	err := c.checkDependencies()
	if err != nil {
		return fmt.Errorf("Failed to check dependencies: %w", err)
	}

	// if an error is returned, disable the usage message
	cmd.SilenceUsage = true

	// Clean up cache directory before doing anything
	err = os.RemoveAll(c.global.flagCacheDir)
	if err != nil {
		return fmt.Errorf("Failed to remove directory %q: %w", c.global.flagCacheDir, err)
	}

	success := false

	err = os.Mkdir(c.global.flagCacheDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", c.global.flagCacheDir, err)
	}

	defer func() {
		if c.global.flagCleanup && !success {
			os.RemoveAll(c.global.flagCacheDir)
		}
	}()

	c.global.sourceDir = filepath.Join(c.global.flagCacheDir, "source")

	// Create source path
	err = os.MkdirAll(c.global.sourceDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", c.global.sourceDir, err)
	}

	// Mount windows ISO
	logger.Infof("Mounting Windows ISO to dir: %q", c.global.sourceDir)
	err = shared.RunCommand(c.global.ctx, nil, nil, "mount", "-t", "udf", "-o", "loop,ro", args[0], c.global.sourceDir)
	if err != nil {
		return fmt.Errorf("Failed to mount %q at %q: %w", args[0], c.global.sourceDir, err)
	}

	c.umounts = append(c.umounts, c.global.sourceDir)

	// Check virtio ISO path
	err = c.checkVirtioISOPath()
	if err != nil {
		return fmt.Errorf("Failed to check virtio ISO Path: %w", err)
	}

	driverPath := filepath.Join(c.global.flagCacheDir, "drivers")
	if !incus.PathExists(driverPath) {
		err := os.MkdirAll(driverPath, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", driverPath, err)
		}
	}

	// Mount driver ISO
	logger.Infof("Mounting driver ISO to dir %q", driverPath)
	err = shared.RunCommand(c.global.ctx, nil, nil, "mount", "-t", "iso9660", "-o", "loop,ro", c.flagDrivers, driverPath)
	if err != nil {
		return fmt.Errorf("Failed to mount %q at %q: %w", c.flagDrivers, driverPath, err)
	}

	c.umounts = append(c.umounts, driverPath)
	success = true
	return nil
}

func (c *cmdRepackWindows) checkVirtioISOPath() (err error) {
	logger := c.global.logger
	virtioISOPath := c.flagDrivers
	if virtioISOPath == "" {
		virtioISOPath = c.defaultDrivers
	}

	if incus.PathExists(virtioISOPath) {
		c.flagDrivers = virtioISOPath
		return
	}

	virtioISOPath = filepath.Join(c.global.flagSourcesDir, "windows", c.defaultDrivers)
	if incus.PathExists(virtioISOPath) {
		c.flagDrivers = virtioISOPath
		return
	}

	// Download vioscsi driver
	virtioURL := "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/latest-virtio/" + c.defaultDrivers
	err = os.MkdirAll(filepath.Dir(virtioISOPath), 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", filepath.Dir(virtioISOPath), err)
	}

	f, err := os.Create(virtioISOPath)
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %w", virtioISOPath, err)
	}

	removeNeeded := false
	defer func() {
		f.Close()
		if c.global.flagCleanup && removeNeeded {
			os.Remove(virtioISOPath)
		}
	}()

	logger.Info("Downloading drivers ISO")
	_, err = incus.DownloadFileHash(c.global.ctx, http.DefaultClient, "", nil, nil, c.defaultDrivers, virtioURL, "", nil, f)
	if err != nil {
		removeNeeded = true
		return fmt.Errorf("Failed to download %q: %w", virtioURL, err)
	}

	c.flagDrivers = virtioISOPath
	return
}

func (c *cmdRepackWindows) run(cmd *cobra.Command, args []string, overlayDir string) error {
	logger := c.global.logger
	bootWim, err := shared.FindFirstMatch(overlayDir, "sources", "boot.wim")
	if err != nil {
		bootWim, err = shared.FindFirstMatch(overlayDir, "sources", "boot.esd")
		if err != nil {
			return fmt.Errorf("Unable to find boot.wim or boot.esd: %w", err)
		}
	}

	installWim, err := shared.FindFirstMatch(overlayDir, "sources", "install.wim")
	if err != nil {
		installWim, err = shared.FindFirstMatch(overlayDir, "sources", "install.esd")
		if err != nil {
			return fmt.Errorf("Unable to find install.wim or install.esd: %w", err)
		}
	}

	bootWimInfo, err := c.getWimInfo(bootWim)
	if err != nil {
		return fmt.Errorf("Failed to get boot wim info: %w", err)
	}

	installWimInfo, err := c.getWimInfo(installWim)
	if err != nil {
		return fmt.Errorf("Failed to get install wim info: %w", err)
	}

	if c.flagWindowsVersion == "" {
		c.flagWindowsVersion = windows.DetectWindowsVersion(installWimInfo.Name(1))
	}

	if c.flagWindowsArchitecture == "" {
		c.flagWindowsArchitecture = windows.DetectWindowsArchitecture(installWimInfo.Architecture(1))
	}

	if c.flagWindowsVersion == "" {
		return errors.New("Failed to detect Windows version. Please provide the version using the --windows-version flag")
	}

	if c.flagWindowsArchitecture == "" {
		return errors.New("Failed to detect Windows architecture. Please provide the architecture using the --windows-arch flag")
	}

	// This injects the drivers into the installation process
	err = c.modifyWim(bootWim, bootWimInfo)
	if err != nil {
		return fmt.Errorf("Failed to modify wim %q: %w", filepath.Base(bootWim), err)
	}

	// This injects the drivers into the final OS
	err = c.modifyWim(installWim, installWimInfo)
	if err != nil {
		return fmt.Errorf("Failed to modify wim %q: %w", filepath.Base(installWim), err)
	}

	logger.Info("Generating new ISO")
	var stdout strings.Builder
	var software string

	err = shared.RunCommand(c.global.ctx, nil, &stdout, "genisoimage", "--version")
	if err != nil {
		err = shared.RunCommand(c.global.ctx, nil, &stdout, "mkisofs", "--version")
		if err != nil {
			return fmt.Errorf("Failed to determine version of genisoimage or mkisofs: %w", err)
		} else {
			software = "mkisofs"
		}
	} else {
		software = "genisoimage"
	}

	version := strings.Split(stdout.String(), "\n")[0]

	genArgs := []string{"-l", "-iso-level", "4", "-no-emul-boot",
		"-b", "boot/etfsboot.com", "-boot-load-seg", "0",
		"-boot-load-size", "8", "-eltorito-alt-boot"}
	if strings.HasPrefix(version, "mkisofs") {
		genArgs = append(genArgs,
			"-eltorito-platform", "efi", "-no-emul-boot",
			"-b", "efi/microsoft/boot/efisys.bin",
			"-boot-load-size", "1", "-UDF")
	} else {
		genArgs = append(genArgs,
			"--allow-limited-size", "-no-emul-boot",
			"-e", "efi/microsoft/boot/efisys.bin",
			"-boot-load-size", "1", "-udf")
	}

	genArgs = append(genArgs, "-o", args[1], overlayDir)
	err = shared.RunCommand(context.WithValue(c.global.ctx, shared.ContextKeyStderr,
		shared.WriteFunc(func(b []byte) (int, error) {
			for i := range b {
				if b[i] == '\n' {
					b[i] = '\r'
				}
			}

			return os.Stderr.Write(b)
		})),
		nil, nil, software, genArgs...)

	if err != nil {
		return fmt.Errorf("Failed to generate ISO: %w", err)
	}

	return nil
}

func (c *cmdRepackWindows) getWimInfo(wimFile string) (info windows.WimInfo, err error) {
	wimName := filepath.Base(wimFile)
	var buf bytes.Buffer
	err = shared.RunCommand(c.global.ctx, nil, &buf, "wimlib-imagex", "info", wimFile)
	if err != nil {
		err = fmt.Errorf("Failed to retrieve wim %q information: %w", wimName, err)
		return
	}

	info, err = windows.ParseWimInfo(&buf)
	if err != nil {
		err = fmt.Errorf("Failed to parse wim info %s: %w", wimFile, err)
		return
	}

	return
}

func (c *cmdRepackWindows) modifyWim(wimFile string, info windows.WimInfo) (err error) {
	wimName := filepath.Base(wimFile)
	// Injects the drivers
	for idx := 1; idx <= info.ImageCount(); idx++ {
		name := info.Name(idx)
		err = c.modifyWimIndex(wimFile, idx, name)
		if err != nil {
			return fmt.Errorf("Failed to modify index %d=%s of %q: %w", idx, name, wimName, err)
		}
	}
	return
}

func (c *cmdRepackWindows) modifyWimIndex(wimFile string, index int, name string) error {
	wimIndex := strconv.Itoa(index)
	wimPath := filepath.Join(c.global.flagCacheDir, "wim", wimIndex)
	wimName := filepath.Base(wimFile)
	logger := c.global.logger.WithFields(logrus.Fields{"wim": wimName, "idx": wimIndex + ":" + name})
	if !incus.PathExists(wimPath) {
		err := os.MkdirAll(wimPath, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", wimPath, err)
		}
	}

	success := false
	logger.Info("Mounting")
	// Mount wim file
	err := shared.RunCommand(c.global.ctx, nil, nil, "wimlib-imagex", "mountrw", wimFile, wimIndex, wimPath, "--allow-other")
	if err != nil {
		return fmt.Errorf("Failed to mount %q: %w", wimName, err)
	}

	defer func() {
		if !success {
			_ = shared.RunCommand(c.global.ctx, nil, nil, "wimlib-imagex", "unmount", wimPath)
		}
	}()

	dirs, err := c.getWindowsDirectories(wimPath)
	if err != nil {
		return fmt.Errorf("Failed to get required windows directories: %w", err)
	}

	logger.Info("Modifying")
	// Create registry entries and copy files
	err = c.injectDrivers(dirs["inf"], dirs["drivers"], dirs["filerepository"], dirs["config"])
	if err != nil {
		return fmt.Errorf("Failed to inject drivers: %w", err)
	}

	logger.Info("Unmounting")
	err = shared.RunCommand(c.global.ctx, nil, nil, "wimlib-imagex", "unmount", wimPath, "--commit")
	if err != nil {
		return fmt.Errorf("Failed to unmount WIM image %q: %w", wimName, err)
	}

	success = true
	return nil
}

func (c *cmdRepackWindows) checkDependencies() error {
	dependencies := []string{"hivexregedit", "rsync", "wimlib-imagex"}

	for _, dep := range dependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			return fmt.Errorf("Required tool %q is missing", dep)
		}
	}

	_, err := exec.LookPath("genisoimage")
	_, err1 := exec.LookPath("mkisofs")
	if err != nil && err1 != nil {
		return fmt.Errorf("Required tool genisoimage or mkisofs is missing")
	}

	return nil
}

func (c *cmdRepackWindows) getWindowsDirectories(wimPath string) (dirs map[string]string, err error) {
	dirs = map[string]string{}
	dirs["inf"], err = shared.FindFirstMatch(wimPath, "windows", "inf")
	if err != nil {
		return nil, fmt.Errorf("Failed to determine windows/inf path: %w", err)
	}

	dirs["config"], err = shared.FindFirstMatch(wimPath, "windows", "system32", "config")
	if err != nil {
		return nil, fmt.Errorf("Failed to determine windows/system32/config path: %w", err)
	}

	dirs["drivers"], err = shared.FindFirstMatch(wimPath, "windows", "system32", "drivers")
	if err != nil {
		return nil, fmt.Errorf("Failed to determine windows/system32/drivers path: %w", err)
	}

	dirs["filerepository"], err = shared.FindFirstMatch(wimPath, "windows", "system32", "driverstore", "filerepository")
	if err != nil {
		return nil, fmt.Errorf("Failed to determine windows/system32/driverstore/filerepository path: %w", err)
	}

	return
}

func (c *cmdRepackWindows) injectDrivers(infDir, driversDir, filerepositoryDir, configDir string) error {
	logger := c.global.logger

	driverPath := filepath.Join(c.global.flagCacheDir, "drivers")
	i := 0

	driversRegistry := "Windows Registry Editor Version 5.00"
	systemRegistry := "Windows Registry Editor Version 5.00"
	softwareRegistry := "Windows Registry Editor Version 5.00"
	for driverName, driverInfo := range windows.Drivers {
		logger.WithField("driver", driverName).Debug("Injecting driver")
		infFilename := fmt.Sprintf("oem%d.inf", i)
		sourceDir := filepath.Join(driverPath, driverName, c.flagWindowsVersion, c.flagWindowsArchitecture)
		targetBaseDir := filepath.Join(filerepositoryDir, driverInfo.PackageName)
		if !incus.PathExists(targetBaseDir) {
			err := os.MkdirAll(targetBaseDir, 0755)
			if err != nil {
				logger.Error(err)
				return err
			}
		}

		for ext, dir := range map[string]string{"inf": infDir, "cat": driversDir, "dll": driversDir, "sys": driversDir} {
			sourceMatches, err := shared.FindAllMatches(sourceDir, fmt.Sprintf("*.%s", ext))
			if err != nil {
				logger.Debugf("failed to find first match %q %q", driverName, ext)
				continue
			}

			for _, sourcePath := range sourceMatches {
				targetName := filepath.Base(sourcePath)
				targetPath := filepath.Join(targetBaseDir, targetName)
				if err = shared.Copy(sourcePath, targetPath); err != nil {
					return err
				}

				if ext == "cat" {
					continue
				} else if ext == "inf" {
					targetName = infFilename
				}

				targetPath = filepath.Join(dir, targetName)
				if err = shared.Copy(sourcePath, targetPath); err != nil {
					return err
				}
			}
		}

		classGuid, err := windows.ParseDriverClassGuid(driverName, filepath.Join(infDir, infFilename))
		if err != nil {
			return err
		}

		ctx := pongo2.Context{
			"infFile":     infFilename,
			"packageName": driverInfo.PackageName,
			"driverName":  driverName,
			"classGuid":   classGuid,
		}

		// Update Windows DRIVERS registry
		if driverInfo.DriversRegistry != "" {
			tpl, err := pongo2.FromString(driverInfo.DriversRegistry)
			if err != nil {
				return fmt.Errorf("Failed to parse template for driver %q: %w", driverName, err)
			}

			out, err := tpl.Execute(ctx)
			if err != nil {
				return fmt.Errorf("Failed to render template for driver %q: %w", driverName, err)
			}

			driversRegistry = fmt.Sprintf("%s\n\n%s", driversRegistry, out)
		}

		// Update Windows SYSTEM registry
		if driverInfo.SystemRegistry != "" {
			tpl, err := pongo2.FromString(driverInfo.SystemRegistry)
			if err != nil {
				return fmt.Errorf("Failed to parse template for driver %q: %w", driverName, err)
			}

			out, err := tpl.Execute(ctx)
			if err != nil {
				return fmt.Errorf("Failed to render template for driver %q: %w", driverName, err)
			}

			systemRegistry = fmt.Sprintf("%s\n\n%s", systemRegistry, out)
		}

		// Update Windows SOFTWARE registry
		if driverInfo.SoftwareRegistry != "" {
			tpl, err := pongo2.FromString(driverInfo.SoftwareRegistry)
			if err != nil {
				return fmt.Errorf("Failed to parse template for driver %q: %w", driverName, err)
			}

			out, err := tpl.Execute(ctx)
			if err != nil {
				return fmt.Errorf("Failed to render template for driver %q: %w", driverName, err)
			}

			softwareRegistry = fmt.Sprintf("%s\n\n%s", softwareRegistry, out)
		}

		i++
	}

	logger.WithField("hivefile", "DRIVERS").Debug("Updating Windows registry")

	err := shared.RunCommand(c.global.ctx, strings.NewReader(driversRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\DRIVERS'", filepath.Join(configDir, "DRIVERS"))
	if err != nil {
		return fmt.Errorf("Failed to edit Windows DRIVERS registry: %w", err)
	}

	logger.WithField("hivefile", "SYSTEM").Debug("Updating Windows registry")

	err = shared.RunCommand(c.global.ctx, strings.NewReader(systemRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\SYSTEM'", filepath.Join(configDir, "SYSTEM"))
	if err != nil {
		return fmt.Errorf("Failed to edit Windows SYSTEM registry: %w", err)
	}

	logger.WithField("hivefile", "SOFTWARE").Debug("Updating Windows registry")

	err = shared.RunCommand(c.global.ctx, strings.NewReader(softwareRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\SOFTWARE'", filepath.Join(configDir, "SOFTWARE"))
	if err != nil {
		return fmt.Errorf("Failed to edit Windows SOFTWARE registry: %w", err)
	}

	return nil
}

// toHex is a pongo2 filter which converts the provided value to a hex value understood by the Windows registry.
func toHex(in *pongo2.Value, param *pongo2.Value) (out *pongo2.Value, err *pongo2.Error) {
	dst := make([]byte, hex.EncodedLen(len(in.String())))
	hex.Encode(dst, []byte(in.String()))

	var builder strings.Builder

	for i := 0; i < len(dst); i += 2 {
		_, err := builder.Write(dst[i : i+2])
		if err != nil {
			return &pongo2.Value{}, &pongo2.Error{Sender: "filter:toHex", OrigError: err}
		}

		_, err = builder.WriteString(",00,")
		if err != nil {
			return &pongo2.Value{}, &pongo2.Error{Sender: "filter:toHex", OrigError: err}
		}
	}

	return pongo2.AsValue(strings.TrimSuffix(builder.String(), ",")), nil
}
