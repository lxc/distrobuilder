package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
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

	flagDrivers string
	flagVersion string
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

			cleanup, overlayDir, err := getOverlay(c.global.flagCacheDir, c.global.sourceDir)
			if err != nil {
				log.Println("OverlayFS not supported. Unpacking ISO ...")

				overlayDir = filepath.Join(c.global.flagCacheDir, "overlay")

				// Use rsync if overlay doesn't work
				err = shared.RunCommand("rsync", "-a", c.global.sourceDir+"/", overlayDir)
				if err != nil {
					return errors.Wrap(err, "Failed to copy content of Windows ISO")
				}
			}

			if cleanup != nil {
				defer cleanup()
			}

			return c.run(cmd, args, overlayDir)
		},
	}

	cmd.Flags().StringVar(&c.flagDrivers, "drivers", "", "Path to drivers ISO"+"``")
	cmd.Flags().StringVar(&c.flagVersion, "version", "", "Windows version to repack"+"``")

	return cmd
}

// Create rw rootfs in preRun. Point global.sourceDir to the rw rootfs.
func (c *cmdRepackWindows) preRun(cmd *cobra.Command, args []string) error {
	if c.flagVersion == "" {
		detectedVersion := detectWindowsVersion(filepath.Base(args[0]))

		if detectedVersion == "" {
			return fmt.Errorf("Failed to detect Windows version. Please provide the version using the --version flag")
		}

		c.flagVersion = detectedVersion
	} else {
		supportedVersions := []string{"w10", "2k19", "2k12"}

		if !lxd.StringInSlice(c.flagVersion, supportedVersions) {
			return fmt.Errorf("Version must be one of %v", supportedVersions)
		}
	}

	// Check dependencies
	err := c.checkDependencies()
	if err != nil {
		return err
	}

	// if an error is returned, disable the usage message
	cmd.SilenceUsage = true

	// Clean up cache directory before doing anything
	err = os.RemoveAll(c.global.flagCacheDir)
	if err != nil {
		return err
	}

	err = os.Mkdir(c.global.flagCacheDir, 0755)
	if err != nil {
		return err
	}

	c.global.sourceDir = filepath.Join(c.global.flagCacheDir, "source")

	// Create source path
	err = os.MkdirAll(c.global.sourceDir, 0755)
	if err != nil {
		return err
	}

	log.Println("Mounting Windows ISO ...")

	// Mount ISO
	_, err = lxd.RunCommand("mount", "-o", "loop", args[0], c.global.sourceDir)
	if err != nil {
		return err
	}

	return nil
}

func (c *cmdRepackWindows) run(cmd *cobra.Command, args []string, overlayDir string) error {
	defer unix.Unmount(c.global.sourceDir, 0)

	driverPath := filepath.Join(c.global.flagCacheDir, "drivers")
	virtioISOPath := c.flagDrivers

	if virtioISOPath == "" {
		// Download vioscsi driver
		virtioURL := "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/latest-virtio/virtio-win.iso"

		virtioISOPath = filepath.Join(os.TempDir(), "distrobuilder", "virtio-win.iso")

		if !lxd.PathExists(virtioISOPath) {
			err := os.MkdirAll(filepath.Dir(virtioISOPath), 0755)
			if err != nil {
				return err
			}

			f, err := os.Create(virtioISOPath)
			if err != nil {
				return err
			}
			defer f.Close()

			var client http.Client

			log.Println("Downloading drivers ISO ...")

			_, err = lxd.DownloadFileHash(&client, "", nil, nil, "virtio-win.iso", virtioURL, "", nil, f)
			if err != nil {
				return err
			}

			f.Close()
		}
	}

	if !lxd.PathExists(driverPath) {
		err := os.MkdirAll(driverPath, 0755)
		if err != nil {
			return err
		}
	}

	log.Println("Mounting driver ISO ...")

	// Mount driver ISO
	_, err := lxd.RunCommand("mount", "-o", "loop", virtioISOPath, driverPath)
	if err != nil {
		return err
	}
	defer unix.Unmount(driverPath, 0)

	// Modify wim (Windows Imaging Format) files
	bootWim := filepath.Join(overlayDir, "sources", "boot.wim")

	// This injects the drivers into the installation process
	err = c.modifyWim(bootWim, 2)
	if err != nil {
		return errors.Wrapf(err, "Failed to modify index 2 of %q", filepath.Base(bootWim))
	}

	var buf bytes.Buffer
	installWim := filepath.Join(overlayDir, "sources", "install.wim")

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

	// This injects the drivers into the final OS
	for _, idx := range indexes {
		err = c.modifyWim(installWim, idx)
		if err != nil {
			return errors.Wrapf(err, "Failed to modify index %d of %q", idx, filepath.Base(installWim))
		}
	}

	log.Println("Generating new ISO ...")

	_, err = lxd.RunCommand("genisoimage", "--allow-limited-size", "-l", "-no-emul-boot", "-b", "efi/microsoft/boot/efisys.bin", "-o", args[1], overlayDir)
	if err != nil {
		return err
	}

	return nil
}

func (c *cmdRepackWindows) modifyWim(path string, index int) error {
	// Mount VIM file
	wimFile := filepath.Join(path)
	wimPath := filepath.Join(c.global.flagCacheDir, "wim")

	if !lxd.PathExists(wimPath) {
		err := os.MkdirAll(wimPath, 0755)
		if err != nil {
			return err
		}
	}

	_, err := lxd.RunCommand("wimlib-imagex", "mountrw", wimFile, strconv.Itoa(index), wimPath, "--allow-other")
	if err != nil {
		return errors.Wrapf(err, "Failed to mount %q", filepath.Base(wimFile))
	}

	log.Printf("Injecting drivers into %s (index %d)...\n", filepath.Base(path), index)

	// Create registry entries and copy files
	err = c.injectDrivers(wimPath)
	if err != nil {
		lxd.RunCommand("wimlib-imagex", "unmount", wimPath)
		return errors.Wrap(err, "Failed to inject drivers")
	}

	_, err = lxd.RunCommand("wimlib-imagex", "unmount", wimPath, "--commit")
	if err != nil {
		return err
	}

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

func (c *cmdRepackWindows) injectDrivers(wimPath string) error {
	driverPath := filepath.Join(c.global.flagCacheDir, "drivers")
	i := 0

	for driver, info := range windows.Drivers {
		ctx := pongo2.Context{
			"infFile":     fmt.Sprintf("oem%d.inf", i),
			"packageName": info.PackageName,
			"driverName":  driver,
		}

		sourceDir := filepath.Join(driverPath, driver, c.flagVersion, "amd64")
		targetBasePath := filepath.Join(wimPath, "Windows/System32/DriverStore/FileRepository", info.PackageName)

		if !lxd.PathExists(targetBasePath) {
			err := os.MkdirAll(targetBasePath, 0755)
			if err != nil {
				return err
			}
		}

		err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
			ext := filepath.Ext(path)
			targetPath := filepath.Join(targetBasePath, filepath.Base(path))

			// Copy driver files
			if lxd.StringInSlice(ext, []string{".cat", ".dll", ".inf", ".sys"}) {
				err := shared.Copy(path, targetPath)
				if err != nil {
					return errors.Wrapf(err, "Failed to copy %q to %q", filepath.Base(path), targetPath)
				}
			}

			// Copy .inf file
			if ext == ".inf" {
				infPath := filepath.Join(wimPath, "Windows/INF")

				if !lxd.PathExists(infPath) {
					infPath = filepath.Join(wimPath, "Windows/Inf")
				}

				target := filepath.Join(infPath, ctx["infFile"].(string))

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
					return fmt.Errorf("Failed to determine classGUID for driver %q", driver)
				}
			}

			// Copy .sys and .dll files
			if ext == ".dll" || ext == ".sys" {
				target := filepath.Join(wimPath, "Windows/System32/drivers", filepath.Base(path))

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

			err = lxd.RunCommandWithFds(strings.NewReader(out), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\DRIVERS'", filepath.Join(wimPath, "/Windows/System32/config/DRIVERS"))
			if err != nil {
				return errors.Wrapf(err, "Failed to edit Windows DRIVERS registry for driver %q", driver)
			}
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

			err = lxd.RunCommandWithFds(strings.NewReader(out), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\SYSTEM'", filepath.Join(wimPath, "/Windows/System32/config/SYSTEM"))
			if err != nil {
				return errors.Wrapf(err, "Failed to edit Windows SYSTEM registry for driver %q", driver)
			}
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

			err = lxd.RunCommandWithFds(strings.NewReader(out), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\SOFTWARE'", filepath.Join(wimPath, "/Windows/System32/config/SOFTWARE"))
			if err != nil {
				return errors.Wrapf(err, "Failed to edit Windows SOFTWARE registry for driver %q", driver)
			}
		}

		i++
	}

	return nil
}

func detectWindowsVersion(fileName string) string {
	aliases := map[string][]string{
		"w10":  {"w10", "win10", "windows.?10"},
		"2k19": {"2k19", "w2k19", "win2k19", "windows.?server.?2019"},
		"2k12": {"2k12", "w2k12", "win2k12", "windows.?server.?2012"},
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
