package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	defaultDrivers         string
	supportedVersions      []string
	supportedArchitectures []string
	umounts                []string
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
	c.supportedVersions = []string{"w11", "w10", "2k19", "2k12", "2k16", "2k22"}
	c.supportedArchitectures = []string{"amd64", "ARM64"}
	cmd.Flags().StringVar(&c.flagDrivers, "drivers", c.defaultDrivers, "Path to virtio windowns drivers ISO file"+"``")
	cmd.Flags().StringVar(&c.flagWindowsVersion, "windows-version", "",
		"Windows version to repack, must be one of ["+strings.Join(c.supportedVersions, ", ")+"]``")
	cmd.Flags().StringVar(&c.flagWindowsArchitecture, "windows-arch", "",
		"Windows architecture to repack, must be one of ["+strings.Join(c.supportedArchitectures, ", ")+"]``")

	return cmd
}

// Create rw rootfs in preRun. Point global.sourceDir to the rw rootfs.
func (c *cmdRepackWindows) preRun(cmd *cobra.Command, args []string) error {
	logger := c.global.logger

	if c.flagWindowsVersion == "" {
		detectedVersion := detectWindowsVersion(filepath.Base(args[0]))

		if detectedVersion == "" {
			return errors.New("Failed to detect Windows version. Please provide the version using the --windows-version flag")
		}

		c.flagWindowsVersion = detectedVersion
	} else {
		if !slices.Contains(c.supportedVersions, c.flagWindowsVersion) {
			return fmt.Errorf("Version must be one of %v", c.supportedVersions)
		}
	}

	if c.flagWindowsArchitecture == "" {
		detectedArchitecture := detectWindowsArchitecture(filepath.Base(args[0]))

		if detectedArchitecture == "" {
			return errors.New("Failed to detect Windows architecture. Please provide the architecture using the --windows-arch flag")
		}

		c.flagWindowsArchitecture = detectedArchitecture
	} else {
		if !slices.Contains(c.supportedArchitectures, c.flagWindowsArchitecture) {
			return fmt.Errorf("Architecture must be one of %v", c.supportedArchitectures)
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
		return fmt.Errorf("Unable to find boot.wim: %w", err)
	}

	installWim, err := shared.FindFirstMatch(overlayDir, "sources", "install.wim")
	if err != nil {
		return fmt.Errorf("Unable to find install.wim: %w", err)
	}

	var buf bytes.Buffer

	err = shared.RunCommand(c.global.ctx, nil, &buf, "wimlib-imagex", "info", installWim)
	if err != nil {
		return fmt.Errorf("Failed to retrieve wim file information: %w", err)
	}

	indexes := []int{}
	scanner := bufio.NewScanner(&buf)

	for scanner.Scan() {
		text := scanner.Text()

		if strings.HasPrefix(text, "Index") {
			fields := strings.Split(text, " ")

			index, err := strconv.Atoi(fields[len(fields)-1])
			if err != nil {
				return fmt.Errorf("Failed to determine wim file indexes: %w", err)
			}

			indexes = append(indexes, index)
		}
	}

	// This injects the drivers into the installation process
	err = c.modifyWim(bootWim, 2)
	if err != nil {
		return fmt.Errorf("Failed to modify index 2 of %q: %w", filepath.Base(bootWim), err)
	}

	// This injects the drivers into the final OS
	for _, idx := range indexes {
		err = c.modifyWim(installWim, idx)
		if err != nil {
			return fmt.Errorf("Failed to modify index %d of %q: %w", idx, filepath.Base(installWim), err)
		}
	}

	logger.Info("Generating new ISO")
	var stdout strings.Builder

	err = shared.RunCommand(c.global.ctx, nil, &stdout, "genisoimage", "--version")
	if err != nil {
		return fmt.Errorf("Failed to determine version of genisoimage: %w", err)
	}

	version := strings.Split(stdout.String(), "\n")[0]
	genArgs := []string{"--allow-limited-size"}
	if strings.HasPrefix(version, "mkisofs") {
		genArgs = []string{"-iso-level", "3"}
	}

	genArgs = append(genArgs, "-input-charset", "utf-8", "-l", "-no-emul-boot",
		"-b", "efi/microsoft/boot/efisys.bin", "-o", args[1], overlayDir)
	err = shared.RunCommand(context.WithValue(c.global.ctx, shared.ContextKeyStderr,
		shared.WriteFunc(func(b []byte) (int, error) {
			for i := range b {
				if b[i] == '\n' {
					b[i] = '\r'
				}
			}

			return os.Stderr.Write(b)
		})),
		nil, nil, "genisoimage", genArgs...)

	if err != nil {
		return fmt.Errorf("Failed to generate ISO: %w", err)
	}

	return nil
}

func (c *cmdRepackWindows) modifyWim(path string, index int) error {
	logger := c.global.logger

	// Mount wim file
	wimFile := filepath.Join(path)
	wimIndex := strconv.Itoa(index)
	wimPath := filepath.Join(c.global.flagCacheDir, "wim", wimIndex)

	if !incus.PathExists(wimPath) {
		err := os.MkdirAll(wimPath, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", wimPath, err)
		}
	}

	success := false

	err := shared.RunCommand(c.global.ctx, nil, nil, "wimlib-imagex", "mountrw", wimFile, wimIndex, wimPath, "--allow-other")
	if err != nil {
		return fmt.Errorf("Failed to mount %q: %w", filepath.Base(wimFile), err)
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

	logger.WithFields(logrus.Fields{"file": filepath.Base(path), "index": index}).Info("Modifying WIM file")

	// Create registry entries and copy files
	err = c.injectDrivers(dirs)
	if err != nil {
		return fmt.Errorf("Failed to inject drivers: %w", err)
	}

	err = shared.RunCommand(c.global.ctx, nil, nil, "wimlib-imagex", "unmount", wimPath, "--commit")
	if err != nil {
		return fmt.Errorf("Failed to unmount WIM image: %w", err)
	}

	success = true
	return nil
}

func (c *cmdRepackWindows) checkDependencies() error {
	dependencies := []string{"genisoimage", "hivexregedit", "rsync", "wimlib-imagex"}

	for _, dep := range dependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			return fmt.Errorf("Required tool %q is missing", dep)
		}
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

func (c *cmdRepackWindows) injectDrivers(dirs map[string]string) error {
	logger := c.global.logger

	driverPath := filepath.Join(c.global.flagCacheDir, "drivers")
	i := 0

	driversRegistry := "Windows Registry Editor Version 5.00"
	systemRegistry := "Windows Registry Editor Version 5.00"
	softwareRegistry := "Windows Registry Editor Version 5.00"

	for driver, info := range windows.Drivers {
		logger.WithField("driver", driver).Debug("Injecting driver")

		ctx := pongo2.Context{
			"infFile":     fmt.Sprintf("oem%d.inf", i),
			"packageName": info.PackageName,
			"driverName":  driver,
		}

		sourceDir := filepath.Join(driverPath, driver, c.flagWindowsVersion, c.flagWindowsArchitecture)
		targetBasePath := filepath.Join(dirs["filerepository"], info.PackageName)

		if !incus.PathExists(targetBasePath) {
			err := os.MkdirAll(targetBasePath, 0755)
			if err != nil {
				return fmt.Errorf("Failed to create directory %q: %w", targetBasePath, err)
			}
		}

		err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			ext := filepath.Ext(path)
			targetPath := filepath.Join(targetBasePath, filepath.Base(path))

			// Copy driver files
			if slices.Contains([]string{".cat", ".dll", ".inf", ".sys"}, ext) {
				logger.WithFields(logrus.Fields{"src": path, "dest": targetPath}).Debug("Copying file")

				err := shared.Copy(path, targetPath)
				if err != nil {
					return fmt.Errorf("Failed to copy %q to %q: %w", filepath.Base(path), targetPath, err)
				}
			}

			// Copy .inf file
			if ext == ".inf" {
				target := filepath.Join(dirs["inf"], ctx["infFile"].(string))
				logger.WithFields(logrus.Fields{"src": path, "dest": target}).Debug("Copying file")

				err := shared.Copy(path, target)
				if err != nil {
					return fmt.Errorf("Failed to copy %q to %q: %w", filepath.Base(path), target, err)
				}

				// Retrieve the ClassGuid which is needed for the Windows registry entries.
				file, err := os.Open(path)
				if err != nil {
					return fmt.Errorf("Failed to open %s: %w", path, err)
				}

				re := regexp.MustCompile(`(?i)^ClassGuid[ ]*=[ ]*(.+)$`)
				scanner := bufio.NewScanner(file)

				for scanner.Scan() {
					matches := re.FindStringSubmatch(scanner.Text())

					if len(matches) > 0 {
						ctx["classGuid"] = strings.TrimSpace(matches[1])
					}
				}

				file.Close()

				_, ok := ctx["classGuid"]
				if !ok {
					return fmt.Errorf("Failed to determine classGUID for driver %q", driver)
				}
			}

			// Copy .sys and .dll files
			if ext == ".dll" || ext == ".sys" {
				target := filepath.Join(dirs["drivers"], filepath.Base(path))
				logger.WithFields(logrus.Fields{"src": path, "dest": target}).Debug("Copying file")

				err := shared.Copy(path, target)
				if err != nil {
					return fmt.Errorf("Failed to copy %q to %q: %w", filepath.Base(path), target, err)
				}
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("Failed to copy driver files: %w", err)
		}

		// Update Windows DRIVERS registry
		if info.DriversRegistry != "" {
			tpl, err := pongo2.FromString(info.DriversRegistry)
			if err != nil {
				return fmt.Errorf("Failed to parse template for driver %q: %w", driver, err)
			}

			out, err := tpl.Execute(ctx)
			if err != nil {
				return fmt.Errorf("Failed to render template for driver %q: %w", driver, err)
			}

			driversRegistry = fmt.Sprintf("%s\n\n%s", driversRegistry, out)
		}

		// Update Windows SYSTEM registry
		if info.SystemRegistry != "" {
			tpl, err := pongo2.FromString(info.SystemRegistry)
			if err != nil {
				return fmt.Errorf("Failed to parse template for driver %q: %w", driver, err)
			}

			out, err := tpl.Execute(ctx)
			if err != nil {
				return fmt.Errorf("Failed to render template for driver %q: %w", driver, err)
			}

			systemRegistry = fmt.Sprintf("%s\n\n%s", systemRegistry, out)
		}

		// Update Windows SOFTWARE registry
		if info.SoftwareRegistry != "" {
			tpl, err := pongo2.FromString(info.SoftwareRegistry)
			if err != nil {
				return fmt.Errorf("Failed to parse template for driver %q: %w", driver, err)
			}

			out, err := tpl.Execute(ctx)
			if err != nil {
				return fmt.Errorf("Failed to render template for driver %q: %w", driver, err)
			}

			softwareRegistry = fmt.Sprintf("%s\n\n%s", softwareRegistry, out)
		}

		i++
	}

	logger.WithField("hivefile", "DRIVERS").Debug("Updating Windows registry")

	err := shared.RunCommand(c.global.ctx, strings.NewReader(driversRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\DRIVERS'", filepath.Join(dirs["config"], "DRIVERS"))
	if err != nil {
		return fmt.Errorf("Failed to edit Windows DRIVERS registry: %w", err)
	}

	logger.WithField("hivefile", "SYSTEM").Debug("Updating Windows registry")

	err = shared.RunCommand(c.global.ctx, strings.NewReader(systemRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\SYSTEM'", filepath.Join(dirs["config"], "SYSTEM"))
	if err != nil {
		return fmt.Errorf("Failed to edit Windows SYSTEM registry: %w", err)
	}

	logger.WithField("hivefile", "SOFTWARE").Debug("Updating Windows registry")

	err = shared.RunCommand(c.global.ctx, strings.NewReader(softwareRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\SOFTWARE'", filepath.Join(dirs["config"], "SOFTWARE"))
	if err != nil {
		return fmt.Errorf("Failed to edit Windows SOFTWARE registry: %w", err)
	}

	return nil
}

func detectWindowsVersion(fileName string) string {
	aliases := map[string][]string{
		"w11":  {"w11", "win11", "windows.?11"},
		"w10":  {"w10", "win10", "windows.?10"},
		"2k19": {"2k19", "w2k19", "win2k19", "windows.?server.?2019"},
		"2k12": {"2k12", "w2k12", "win2k12", "windows.?server.?2012"},
		"2k16": {"2k16", "w2k16", "win2k16", "windows.?server.?2016"},
		"2k22": {"2k22", "w2k22", "win2k22", "windows.?server.?2022"},
	}

	for k, v := range aliases {
		for _, alias := range v {
			if regexp.MustCompile(fmt.Sprintf("(?i)%s", alias)).MatchString(fileName) {
				return k
			}
		}
	}

	return ""
}

func detectWindowsArchitecture(fileName string) string {
	aliases := map[string][]string{
		"amd64": {"amd64", "x64"},
		"ARM64": {"arm64"},
	}

	for k, v := range aliases {
		for _, alias := range v {
			if regexp.MustCompile(fmt.Sprintf("(?i)%s", alias)).MatchString(fileName) {
				return k
			}
		}
	}

	return ""
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
