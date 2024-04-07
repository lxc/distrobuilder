package main

import (
	"bufio"
	"bytes"
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
			defer func() {
				_ = unix.Unmount(c.global.sourceDir, 0)
			}()

			sourceDir := filepath.Dir(args[0])
			targetDir := filepath.Dir(args[1])

			// If either the source or the target are located on a FUSE filesystem, disable overlay
			// as it doesn't play well with wimlib-imagex.
			for _, dir := range []string{sourceDir, targetDir} {
				var stat unix.Statfs_t

				err := unix.Statfs(dir, &stat)
				if err != nil {
					c.global.logger.WithFields(logrus.Fields{"dir": dir, "err": err}).Warn("Failed to get directory information")
					continue
				}

				// Since there's no magic number for virtiofs, we need to check FUSE_SUPER_MAGIC (which is not defined in the unix package).
				if stat.Type == 0x65735546 {
					c.global.logger.Warn("FUSE filesystem detected, disabling overlay")
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

	cmd.Flags().StringVar(&c.flagDrivers, "drivers", "", "Path to drivers ISO"+"``")
	cmd.Flags().StringVar(&c.flagWindowsVersion, "windows-version", "", "Windows version to repack"+"``")
	cmd.Flags().StringVar(&c.flagWindowsArchitecture, "windows-arch", "", "Windows architecture to repack"+"``")

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
		supportedVersions := []string{"w11", "w10", "2k19", "2k12", "2k16", "2k22"}

		if !slices.Contains(supportedVersions, c.flagWindowsVersion) {
			return fmt.Errorf("Version must be one of %v", supportedVersions)
		}
	}

	if c.flagWindowsArchitecture == "" {
		detectedArchitecture := detectWindowsArchitecture(filepath.Base(args[0]))

		if detectedArchitecture == "" {
			return errors.New("Failed to detect Windows architecture. Please provide the architecture using the --windows-arch flag")
		}

		c.flagWindowsArchitecture = detectedArchitecture
	} else {
		supportedArchitectures := []string{"amd64", "ARM64"}

		if !slices.Contains(supportedArchitectures, c.flagWindowsArchitecture) {
			return fmt.Errorf("Architecture must be one of %v", supportedArchitectures)
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

	logger.Info("Mounting Windows ISO")

	// Mount ISO
	err = shared.RunCommand(c.global.ctx, nil, nil, "mount", "-t", "udf", "-o", "loop", args[0], c.global.sourceDir)
	if err != nil {
		return fmt.Errorf("Failed to mount %q at %q: %w", args[0], c.global.sourceDir, err)
	}

	success = true
	return nil
}

func (c *cmdRepackWindows) run(cmd *cobra.Command, args []string, overlayDir string) error {
	logger := c.global.logger

	driverPath := filepath.Join(c.global.flagCacheDir, "drivers")
	virtioISOPath := c.flagDrivers

	if virtioISOPath == "" {
		// Download vioscsi driver
		virtioURL := "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/latest-virtio/virtio-win.iso"

		virtioISOPath = filepath.Join(c.global.flagSourcesDir, "windows", "virtio-win.iso")

		if !incus.PathExists(virtioISOPath) {
			err := os.MkdirAll(filepath.Dir(virtioISOPath), 0755)
			if err != nil {
				return fmt.Errorf("Failed to create directory %q: %w", filepath.Dir(virtioISOPath), err)
			}

			f, err := os.Create(virtioISOPath)
			if err != nil {
				return fmt.Errorf("Failed to create file %q: %w", virtioISOPath, err)
			}

			defer f.Close()

			var client http.Client

			logger.Info("Downloading drivers ISO")

			_, err = incus.DownloadFileHash(c.global.ctx, &client, "", nil, nil, "virtio-win.iso", virtioURL, "", nil, f)
			if err != nil {
				f.Close()
				os.Remove(virtioISOPath)
				return fmt.Errorf("Failed to download %q: %w", virtioURL, err)
			}

			f.Close()
		}
	}

	if !incus.PathExists(driverPath) {
		err := os.MkdirAll(driverPath, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", driverPath, err)
		}
	}

	logger.Info("Mounting driver ISO")

	// Mount driver ISO
	err := shared.RunCommand(c.global.ctx, nil, nil, "mount", "-t", "iso9660", "-o", "loop", virtioISOPath, driverPath)
	if err != nil {
		return fmt.Errorf("Failed to mount %q at %q: %w", virtioISOPath, driverPath, err)
	}

	defer func() {
		_ = unix.Unmount(driverPath, 0)
	}()

	var sourcesDir string

	entries, err := os.ReadDir(overlayDir)
	if err != nil {
		return fmt.Errorf("Failed to read directory %q: %w", overlayDir, err)
	}

	for _, entry := range entries {
		if strings.ToLower(entry.Name()) == "sources" {
			sourcesDir = filepath.Join(overlayDir, entry.Name())
			break
		}
	}

	entries, err = os.ReadDir(sourcesDir)
	if err != nil {
		return fmt.Errorf("Failed to read directory %q: %w", sourcesDir, err)
	}

	var bootWim string
	var installWim string

	// Find boot.wim and install.wim but consider their case.
	for _, entry := range entries {
		if bootWim != "" && installWim != "" {
			break
		}

		if strings.ToLower(entry.Name()) == "boot.wim" {
			bootWim = filepath.Join(sourcesDir, entry.Name())
			continue
		}

		if strings.ToLower(entry.Name()) == "install.wim" {
			installWim = filepath.Join(sourcesDir, entry.Name())
			continue
		}
	}

	if bootWim == "" {
		return errors.New("Unable to find boot.wim")
	}

	if installWim == "" {
		return errors.New("Unable to find install.wim")
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

	if strings.HasPrefix(version, "mkisofs") {
		err = shared.RunCommand(c.global.ctx, nil, nil, "genisoimage", "-iso-level", "3", "-l", "-no-emul-boot", "-b", "efi/microsoft/boot/efisys.bin", "-o", args[1], overlayDir)
	} else {
		err = shared.RunCommand(c.global.ctx, nil, nil, "genisoimage", "--allow-limited-size", "-l", "-no-emul-boot", "-b", "efi/microsoft/boot/efisys.bin", "-o", args[1], overlayDir)
	}

	if err != nil {
		return fmt.Errorf("Failed to generate ISO: %w", err)
	}

	return nil
}

func (c *cmdRepackWindows) modifyWim(path string, index int) error {
	logger := c.global.logger

	// Mount VIM file
	wimFile := filepath.Join(path)
	wimPath := filepath.Join(c.global.flagCacheDir, "wim")

	if !incus.PathExists(wimPath) {
		err := os.MkdirAll(wimPath, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", wimPath, err)
		}
	}

	success := false

	err := shared.RunCommand(c.global.ctx, nil, nil, "wimlib-imagex", "mountrw", wimFile, strconv.Itoa(index), wimPath, "--allow-other")
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

	if dirs["filerepository"] == "" {
		return errors.New("Failed to determine windows/system32/driverstore/filerepository path")
	}

	if dirs["inf"] == "" {
		return errors.New("Failed to determine windows/inf path")
	}

	if dirs["config"] == "" {
		return errors.New("Failed to determine windows/system32/config path")
	}

	if dirs["drivers"] == "" {
		return errors.New("Failed to determine windows/system32/drivers path")
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

func (c *cmdRepackWindows) getWindowsDirectories(wimPath string) (map[string]string, error) {
	windowsPath := ""
	system32Path := ""
	driverStorePath := ""
	dirs := make(map[string]string)

	entries, err := os.ReadDir(wimPath)
	if err != nil {
		return nil, err
	}

	// Get windows directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if regexp.MustCompile(`^(?i)windows$`).MatchString(entry.Name()) {
			windowsPath = filepath.Join(wimPath, entry.Name())
			break
		}
	}

	entries, err = os.ReadDir(windowsPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if dirs["inf"] != "" && system32Path != "" {
			break
		}

		if !entry.IsDir() {
			continue
		}

		if regexp.MustCompile(`^(?i)inf$`).MatchString(entry.Name()) {
			dirs["inf"] = filepath.Join(windowsPath, entry.Name())
			continue
		}

		if regexp.MustCompile(`^(?i)system32$`).MatchString(entry.Name()) {
			system32Path = filepath.Join(windowsPath, entry.Name())
		}
	}

	entries, err = os.ReadDir(system32Path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if dirs["config"] != "" && dirs["drivers"] != "" && driverStorePath != "" {
			break
		}

		if !entry.IsDir() {
			continue
		}

		if regexp.MustCompile(`^(?i)config$`).MatchString(entry.Name()) {
			dirs["config"] = filepath.Join(system32Path, entry.Name())
			continue
		}

		if regexp.MustCompile(`^(?i)drivers$`).MatchString(entry.Name()) {
			dirs["drivers"] = filepath.Join(system32Path, entry.Name())
			continue
		}

		if regexp.MustCompile(`^(?i)driverstore$`).MatchString(entry.Name()) {
			driverStorePath = filepath.Join(system32Path, entry.Name())
		}
	}

	entries, err = os.ReadDir(driverStorePath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if regexp.MustCompile(`^(?i)filerepository$`).MatchString(entry.Name()) {
			dirs["filerepository"] = filepath.Join(driverStorePath, entry.Name())
			break
		}
	}

	return dirs, nil
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
