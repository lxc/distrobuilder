package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/flosch/pongo2"
	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/windows"
)

type cmdRepackWindows struct {
	cmd    *cobra.Command
	global *cmdGlobal

	flagDrivers        string
	flagWindowsVersion string
}

func init() {
	// Filters should be registered in the init() function
	pongo2.RegisterFilter("toHex", toHex)
}

func (c *cmdRepackWindows) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repack-windows <source-iso> <target-iso> [--drivers=DRIVERS]",
		Short:   "Repack Windows ISO with drivers included",
		Args:    cobra.ExactArgs(2),
		PreRunE: c.preRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer unix.Unmount(c.global.sourceDir, 0)

			sourceDir := filepath.Dir(args[0])
			targetDir := filepath.Dir(args[1])

			// If either the source or the target are located on a FUSE filesystem, disable overlay
			// as it doesn't play well with wimlib-imagex.
			for _, dir := range []string{sourceDir, targetDir} {
				var stat unix.Statfs_t

				err := unix.Statfs(dir, &stat)
				if err != nil {
					c.global.logger.Warnw("Failed to get directory information", "directory", dir, "err", err)
					continue
				}

				// Since there's no magic number for virtiofs, we need to check FUSE_SUPER_MAGIC (which is not defined in the unix package).
				if stat.Type == 0x65735546 {
					c.global.logger.Warnw("FUSE filesystem detected, disabling overlay")
					c.global.flagDisableOverlay = true
					break
				}
			}

			overlayDir, cleanup, err := c.global.getOverlayDir()
			if err != nil {
				return errors.Wrap(err, "Failed to get overlay directory")
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

	return cmd
}

// Create rw rootfs in preRun. Point global.sourceDir to the rw rootfs.
func (c *cmdRepackWindows) preRun(cmd *cobra.Command, args []string) error {
	logger := c.global.logger

	if c.flagWindowsVersion == "" {
		detectedVersion := detectWindowsVersion(filepath.Base(args[0]))

		if detectedVersion == "" {
			return errors.Errorf("Failed to detect Windows version. Please provide the version using the --windows-version flag")
		}

		c.flagWindowsVersion = detectedVersion
	} else {
		supportedVersions := []string{"w10", "2k19", "2k12", "2k16"}

		if !lxd.StringInSlice(c.flagWindowsVersion, supportedVersions) {
			return errors.Errorf("Version must be one of %v", supportedVersions)
		}
	}

	// Check dependencies
	err := c.checkDependencies()
	if err != nil {
		return errors.Wrap(err, "Failed to check dependencies")
	}

	// if an error is returned, disable the usage message
	cmd.SilenceUsage = true

	// Clean up cache directory before doing anything
	err = os.RemoveAll(c.global.flagCacheDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to remove directory %q", c.global.flagCacheDir)
	}

	success := false

	err = os.Mkdir(c.global.flagCacheDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "Failed to create directory %q", c.global.flagCacheDir)
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
		return errors.Wrapf(err, "Failed to create directory %q", c.global.sourceDir)
	}

	logger.Info("Mounting Windows ISO")

	// Mount ISO
	_, err = lxd.RunCommand("mount", "-o", "loop", args[0], c.global.sourceDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to mount %q at %q", args[0], c.global.sourceDir)
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

		virtioISOPath = filepath.Join(os.TempDir(), "distrobuilder", "virtio-win.iso")

		if !lxd.PathExists(virtioISOPath) {
			err := os.MkdirAll(filepath.Dir(virtioISOPath), 0755)
			if err != nil {
				return errors.Wrapf(err, "Failed to create directory %q", filepath.Dir(virtioISOPath))
			}

			f, err := os.Create(virtioISOPath)
			if err != nil {
				return errors.Wrapf(err, "Failed to create file %q", virtioISOPath)
			}
			defer f.Close()

			var client http.Client

			logger.Info("Downloading drivers ISO")

			_, err = lxd.DownloadFileHash(&client, "", nil, nil, "virtio-win.iso", virtioURL, "", nil, f)
			if err != nil {
				return errors.Wrapf(err, "Failed to download %q", virtioURL)
			}

			f.Close()
		}
	}

	if !lxd.PathExists(driverPath) {
		err := os.MkdirAll(driverPath, 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to create directory %q", driverPath)
		}
	}

	logger.Info("Mounting driver ISO")

	// Mount driver ISO
	_, err := lxd.RunCommand("mount", "-o", "loop", virtioISOPath, driverPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to mount %q at %q", virtioISOPath, driverPath)
	}
	defer unix.Unmount(driverPath, 0)

	var sourcesDir string

	entries, err := ioutil.ReadDir(overlayDir)

	for _, entry := range entries {
		if strings.ToLower(entry.Name()) == "sources" {
			sourcesDir = filepath.Join(overlayDir, entry.Name())
			break
		}
	}

	entries, err = ioutil.ReadDir(sourcesDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to read directory %q", sourcesDir)
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
		return errors.Errorf("Unable to find boot.wim")
	}

	if installWim == "" {
		return errors.Errorf("Unable to find install.wim")
	}

	var buf bytes.Buffer

	err = lxd.RunCommandWithFds(nil, &buf, "wimlib-imagex", "info", installWim)
	if err != nil {
		return errors.Wrap(err, "Failed to retrieve wim file information")
	}

	indexes := []int{}
	scanner := bufio.NewScanner(&buf)

	for scanner.Scan() {
		text := scanner.Text()

		if strings.HasPrefix(text, "Index") {
			fields := strings.Split(text, " ")

			index, err := strconv.Atoi(fields[len(fields)-1])
			if err != nil {
				return errors.Wrap(err, "Failed to determine wim file indexes")
			}
			indexes = append(indexes, index)
		}
	}

	// This injects the drivers into the installation process
	err = c.modifyWim(bootWim, 2)
	if err != nil {
		return errors.Wrapf(err, "Failed to modify index 2 of %q", filepath.Base(bootWim))
	}

	// This injects the drivers into the final OS
	for _, idx := range indexes {
		err = c.modifyWim(installWim, idx)
		if err != nil {
			return errors.Wrapf(err, "Failed to modify index %d of %q", idx, filepath.Base(installWim))
		}
	}

	logger.Info("Generating new ISO")

	stdout, err := lxd.RunCommand("genisoimage", "--version")
	if err != nil {
		return errors.Wrap(err, "Failed to determine version of genisoimage")
	}

	version := strings.Split(stdout, "\n")[0]

	if strings.HasPrefix(version, "mkisofs") {
		_, err = lxd.RunCommand("genisoimage", "-iso-level", "3", "-l", "-no-emul-boot", "-b", "efi/microsoft/boot/efisys.bin", "-o", args[1], overlayDir)
	} else {
		_, err = lxd.RunCommand("genisoimage", "--allow-limited-size", "-l", "-no-emul-boot", "-b", "efi/microsoft/boot/efisys.bin", "-o", args[1], overlayDir)
	}
	if err != nil {
		return errors.Wrap(err, "Failed to generate ISO")
	}

	return nil
}

func (c *cmdRepackWindows) modifyWim(path string, index int) error {
	logger := c.global.logger

	// Mount VIM file
	wimFile := filepath.Join(path)
	wimPath := filepath.Join(c.global.flagCacheDir, "wim")

	if !lxd.PathExists(wimPath) {
		err := os.MkdirAll(wimPath, 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to create directory %q", wimPath)
		}
	}

	success := false

	_, err := lxd.RunCommand("wimlib-imagex", "mountrw", wimFile, strconv.Itoa(index), wimPath, "--allow-other")
	if err != nil {
		return errors.Wrapf(err, "Failed to mount %q", filepath.Base(wimFile))
	}
	defer func() {
		if !success {
			lxd.RunCommand("wimlib-imagex", "unmount", wimPath)
		}
	}()

	dirs, err := c.getWindowsDirectories(wimPath)
	if err != nil {
		return errors.Wrap(err, "Failed to get required windows directories")
	}

	if dirs["filerepository"] == "" {
		return errors.Errorf("Failed to determine windows/system32/driverstore/filerepository path")
	}

	if dirs["inf"] == "" {
		return errors.Errorf("Failed to determine windows/inf path")
	}

	if dirs["config"] == "" {
		return errors.Errorf("Failed to determine windows/system32/config path")
	}

	if dirs["drivers"] == "" {
		return errors.Errorf("Failed to determine windows/system32/drivers path")
	}

	logger.Infow("Modifying WIM file", "file", filepath.Base(path), "index", index)

	// Create registry entries and copy files
	err = c.injectDrivers(dirs)
	if err != nil {
		return errors.Wrap(err, "Failed to inject drivers")
	}

	_, err = lxd.RunCommand("wimlib-imagex", "unmount", wimPath, "--commit")
	if err != nil {
		return errors.Wrap(err, "Failed to unmount WIM image")
	}

	success = true
	return nil
}

func (c *cmdRepackWindows) checkDependencies() error {
	dependencies := []string{"genisoimage", "hivexregedit", "rsync", "wimlib-imagex"}

	for _, dep := range dependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			return errors.Errorf("Required tool %q is missing", dep)
		}
	}

	return nil
}

func (c *cmdRepackWindows) getWindowsDirectories(wimPath string) (map[string]string, error) {
	windowsPath := ""
	system32Path := ""
	driverStorePath := ""
	dirs := make(map[string]string)

	entries, err := ioutil.ReadDir(wimPath)
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

	entries, err = ioutil.ReadDir(windowsPath)
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

	entries, err = ioutil.ReadDir(system32Path)
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

	entries, err = ioutil.ReadDir(driverStorePath)
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
		logger.Debugw("Injecting driver", "driver", driver)

		ctx := pongo2.Context{
			"infFile":     fmt.Sprintf("oem%d.inf", i),
			"packageName": info.PackageName,
			"driverName":  driver,
		}

		sourceDir := filepath.Join(driverPath, driver, c.flagWindowsVersion, "amd64")
		targetBasePath := filepath.Join(dirs["filerepository"], info.PackageName)

		if !lxd.PathExists(targetBasePath) {
			err := os.MkdirAll(targetBasePath, 0755)
			if err != nil {
				return errors.Wrapf(err, "Failed to create directory %q", targetBasePath)
			}
		}

		err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			ext := filepath.Ext(path)
			targetPath := filepath.Join(targetBasePath, filepath.Base(path))

			// Copy driver files
			if lxd.StringInSlice(ext, []string{".cat", ".dll", ".inf", ".sys"}) {
				logger.Debugw("Copying file", "src", path, "dest", targetPath)

				err := shared.Copy(path, targetPath)
				if err != nil {
					return errors.Wrapf(err, "Failed to copy %q to %q", filepath.Base(path), targetPath)
				}
			}

			// Copy .inf file
			if ext == ".inf" {
				target := filepath.Join(dirs["inf"], ctx["infFile"].(string))
				logger.Debugw("Copying file", "src", path, "dest", target)

				err = shared.Copy(path, target)
				if err != nil {
					return errors.Wrapf(err, "Failed to copy %q to %q", filepath.Base(path), target)
				}

				// Retrieve the ClassGuid which is needed for the Windows registry entries.
				file, err := os.Open(path)
				if err != nil {
					return errors.Wrapf(err, "Failed to open %s", path)
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
					return errors.Errorf("Failed to determine classGUID for driver %q", driver)
				}
			}

			// Copy .sys and .dll files
			if ext == ".dll" || ext == ".sys" {
				target := filepath.Join(dirs["drivers"], filepath.Base(path))
				logger.Debugw("Copying file", "src", path, "dest", target)

				err = shared.Copy(path, target)
				if err != nil {
					return errors.Wrapf(err, "Failed to copy %q to %q", filepath.Base(path), target)
				}
			}

			return nil
		})
		if err != nil {
			return errors.Wrap(err, "Failed to copy driver files")
		}

		// Update Windows DRIVERS registry
		if info.DriversRegistry != "" {
			tpl, err := pongo2.FromString(info.DriversRegistry)
			if err != nil {
				return errors.Wrapf(err, "Failed to parse template for driver %q", driver)
			}

			out, err := tpl.Execute(ctx)
			if err != nil {
				return errors.Wrapf(err, "Failed to render template for driver %q", driver)
			}

			driversRegistry = fmt.Sprintf("%s\n\n%s", driversRegistry, out)
		}

		// Update Windows SYSTEM registry
		if info.SystemRegistry != "" {
			tpl, err := pongo2.FromString(info.SystemRegistry)
			if err != nil {
				return errors.Wrapf(err, "Failed to parse template for driver %q", driver)
			}

			out, err := tpl.Execute(ctx)
			if err != nil {
				return errors.Wrapf(err, "Failed to render template for driver %q", driver)
			}

			systemRegistry = fmt.Sprintf("%s\n\n%s", systemRegistry, out)
		}

		// Update Windows SOFTWARE registry
		if info.SoftwareRegistry != "" {
			tpl, err := pongo2.FromString(info.SoftwareRegistry)
			if err != nil {
				return errors.Wrapf(err, "Failed to parse template for driver %q", driver)
			}

			out, err := tpl.Execute(ctx)
			if err != nil {
				return errors.Wrapf(err, "Failed to render template for driver %q", driver)
			}

			softwareRegistry = fmt.Sprintf("%s\n\n%s", softwareRegistry, out)
		}

		i++
	}

	logger.Debugw("Updating Windows registry", "hivefile", "DRIVERS")

	err := lxd.RunCommandWithFds(strings.NewReader(driversRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\DRIVERS'", filepath.Join(dirs["config"], "DRIVERS"))
	if err != nil {
		return errors.Wrap(err, "Failed to edit Windows DRIVERS registry")
	}

	logger.Debugw("Updating Windows registry", "hivefile", "SYSTEM")

	err = lxd.RunCommandWithFds(strings.NewReader(systemRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\SYSTEM'", filepath.Join(dirs["config"], "SYSTEM"))
	if err != nil {
		return errors.Wrap(err, "Failed to edit Windows SYSTEM registry")
	}

	logger.Debugw("Updating Windows registry", "hivefile", "SOFTWARE")

	err = lxd.RunCommandWithFds(strings.NewReader(softwareRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\SOFTWARE'", filepath.Join(dirs["config"], "SOFTWARE"))
	if err != nil {
		return errors.Wrap(err, "Failed to edit Windows SOFTWARE registry")
	}

	return nil
}

func detectWindowsVersion(fileName string) string {
	aliases := map[string][]string{
		"w10":  {"w10", "win10", "windows.?10"},
		"2k19": {"2k19", "w2k19", "win2k19", "windows.?server.?2019"},
		"2k12": {"2k12", "w2k12", "win2k12", "windows.?server.?2012"},
		"2k16": {"2k16", "w2k16", "win2k16", "windows.?server.?2016"},
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
