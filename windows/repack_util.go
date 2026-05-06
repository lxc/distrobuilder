package windows

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v4"
	incus "github.com/lxc/incus/v7/shared/util"
	"github.com/sirupsen/logrus"

	"github.com/lxc/distrobuilder/v3/shared"
)

type RepackUtil struct {
	cacheDir            string
	ctx                 context.Context
	logger              *logrus.Logger
	windowsVersion      string
	windowsArchitecture string
}

// NewRepackUtil returns a new RepackUtil object.
func NewRepackUtil(cacheDir string, ctx context.Context, logger *logrus.Logger) RepackUtil {
	return RepackUtil{
		cacheDir: cacheDir,
		ctx:      ctx,
		logger:   logger,
	}
}

// SetWindowsVersionArchitecture is a helper function for setting the specific Windows version and architecture.
func (r *RepackUtil) SetWindowsVersionArchitecture(windowsVersion string, windowsArchitecture string) {
	r.windowsVersion = windowsVersion
	r.windowsArchitecture = windowsArchitecture
}

// GetWimInfo returns information about the specified wim file.
func (r *RepackUtil) GetWimInfo(wimFile string) (WimInfo, error) {
	wimName := filepath.Base(wimFile)
	var buf bytes.Buffer
	err := shared.RunCommand(r.ctx, nil, &buf, "wimlib-imagex", "info", wimFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve wim %q information: %w", wimName, err)
	}

	info, err := ParseWimInfo(&buf)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse wim info %s: %w", wimFile, err)
	}

	return info, nil
}

// InjectDriversIntoWim will inject drivers into the specified wim file.
func (r *RepackUtil) InjectDriversIntoWim(wimFile string, info WimInfo, driverPath string) error {
	wimName := filepath.Base(wimFile)
	// Injects the drivers
	for idx := 1; idx <= info.ImageCount(); idx++ {
		name := info.Name(idx)
		err := r.modifyWimIndex(wimFile, idx, name, driverPath)
		if err != nil {
			return fmt.Errorf("Failed to modify index %d=%s of %q: %w", idx, name, wimName, err)
		}
	}

	return nil
}

// InjectDrivers injects drivers from driverPath into the windowsRootPath.
func (r *RepackUtil) InjectDrivers(windowsRootPath string, driverPath string) error {
	dirs, err := r.getWindowsDirectories(windowsRootPath)
	if err != nil {
		return fmt.Errorf("Failed to get required Windows directories under path '%s': %w", windowsRootPath, err)
	}

	logger := r.logger

	driversRegistry := "Windows Registry Editor Version 5.00"
	systemRegistry := "Windows Registry Editor Version 5.00"
	softwareRegistry := "Windows Registry Editor Version 5.00"
	for driverName, driverInfo := range Drivers {
		logger.WithField("driver", driverName).Debug("Injecting driver")
		infFilename := ""
		sourceDir := filepath.Join(driverPath, driverName, r.windowsVersion, r.windowsArchitecture)
		targetBaseDir := filepath.Join(dirs["filerepository"], driverInfo.PackageName)
		if !incus.PathExists(targetBaseDir) {
			err := os.MkdirAll(targetBaseDir, 0o755)
			if err != nil {
				logger.Error(err)
				return err
			}
		}

		// Special cases introduced with viosock.
		// Ideally we should parse the .inf file for DestinationDirs and CopyFiles sections if special cases start adding up.
		for ext, dir := range map[string]string{"svc.exe": dirs["system32"], "lib_x64.dll": dirs["system32"], "lib_x86.dll": dirs["syswow64"]} {
			sourceMatches, err := shared.FindAllMatches(sourceDir, fmt.Sprintf("*%s", ext))
			if err != nil {
				logger.Debugf("failed to find first match %q %q", driverName, ext)
				continue
			}

			for _, sourcePath := range sourceMatches {
				targetName := filepath.Base(sourcePath)
				if strings.HasSuffix(ext, ".dll") {
					idx := strings.LastIndex(targetName, "_")
					if idx < 0 {
						logger.Debugf("Unexpected lib dll for %q: %q", driverName, targetName)
						continue
					}

					targetName = targetName[:idx] + ".dll"
				}

				targetPath := filepath.Join(dir, targetName)
				if err = shared.Copy(sourcePath, targetPath); err != nil {
					return err
				}
			}
		}

		for ext, dir := range map[string]string{"inf": dirs["inf"], "cat": dirs["drivers"], "dll": dirs["drivers"], "exe": dirs["drivers"], "sys": dirs["drivers"]} {
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

				if ext == "cat" || ext == "exe" {
					continue
				} else if ext == "inf" {
					targetName = "oem-" + targetName
					infFilename = targetName
				}

				targetPath = filepath.Join(dir, targetName)

				// Older versions of Windows don't have a DriverDatabase registry, and will try to re-install all newly detected .inf files, expecting all additional files to be in the same directory.
				if slices.Contains(LegacyWindowsVersions, r.windowsVersion) && ext != "inf" {
					if err = shared.Copy(sourcePath, filepath.Join(dirs["inf"], targetName)); err != nil {
						return err
					}
				}

				if err = shared.Copy(sourcePath, targetPath); err != nil {
					return err
				}
			}
		}

		switch r.windowsVersion {
		case "2k3", "xp":
			// Clear existing driver install logs for debugging purposes.
			winDir := filepath.Dir(dirs["inf"])
			_ = shared.Copy(filepath.Join(winDir, "setupapi.log"), filepath.Join(winDir, "setupapi.log.old"))
			_ = os.Remove(filepath.Join(winDir, "setupapi.log"))
		}

		if infFilename == "" {
			logger.WithFields(logrus.Fields{"driver": driverName, "version": r.windowsVersion}).Warn("Skipping driver not supported by Windows version")
			continue
		}

		classGuid, err := ParseDriverClassGuid(driverName, filepath.Join(dirs["inf"], infFilename))
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

		if slices.Contains(LegacyWindowsVersions, r.windowsVersion) {
			if driverInfo.SystemRegistryLegacy != "" {
				tpl, err := pongo2.FromString(driverInfo.SystemRegistryLegacy)
				if err != nil {
					return fmt.Errorf("Failed to parse template for driver %q: %w", driverName, err)
				}

				out, err := tpl.Execute(ctx)
				if err != nil {
					return fmt.Errorf("Failed to render template for driver %q: %w", driverName, err)
				}

				systemRegistry = fmt.Sprintf("%s\n\n%s", systemRegistry, out)
			}
		} else if driverInfo.SystemRegistryDrivers != "" {
			tpl, err := pongo2.FromString(driverInfo.SystemRegistryDrivers)
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
	}

	// Windows 2003 drivers have a signing issue that prevents auto-install.
	// Instead, the new hardware wizard will request manual driver approval on boot.
	//
	// Some .inf files have one or more SourceDisks sections corresponding to external media.
	// To make the approval and installation a bit less painful, we can set the lookup path to C:\Windows\inf
	// so Windows can find them without prompting the user to attach the CDROM.
	if r.windowsVersion == "2k3" || r.windowsVersion == "xp" {
		tpl, err := pongo2.FromString(`
[Microsoft\Windows\CurrentVersion\Setup]
"SourcePath"=hex(7):{{"C:\\WINDOWS\\inf\\" | toHex}},00,00
`)
		if err != nil {
			return fmt.Errorf("Failed to parse registry template: %w", err)
		}

		softwareRegistry, err = tpl.Execute(pongo2.Context{})
		if err != nil {
			return fmt.Errorf("Failed to render template: %w", err)
		}
	}

	err = r.updateRegistry(r.ctx, dirs["config"], "DRIVERS", driversRegistry)
	if err != nil {
		return err
	}

	err = r.updateRegistry(r.ctx, dirs["config"], "SYSTEM", systemRegistry)
	if err != nil {
		return err
	}

	err = r.updateRegistry(r.ctx, dirs["config"], "SOFTWARE", softwareRegistry)
	if err != nil {
		return err
	}

	return nil
}

func (r *RepackUtil) updateRegistry(ctx context.Context, dir string, hive string, updates string) error {
	if r.windowsVersion == "2k3" || r.windowsVersion == "xp" {
		hive = strings.ToLower(hive)
	}

	_, err := os.Stat(filepath.Join(dir, hive))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err != nil {
		r.logger.WithFields(logrus.Fields{"version": r.windowsVersion, "hive": hive}).Warn("Skipping registry updates for unsupported hive")
		return nil
	}

	r.logger.WithField("hivefile", hive).Debug("Updating Windows registry")
	b := bytes.Buffer{}
	err = shared.RunCommand(context.WithValue(ctx, shared.ContextKeyStderr, &b), strings.NewReader(updates), &b, "hivexregedit", "--merge", fmt.Sprintf("--prefix='HKEY_LOCAL_MACHINE\\%s'", hive), filepath.Join(dir, hive))
	if err != nil {
		return fmt.Errorf("Failed to edit Windows %q registry (%q): %w", hive, b.String(), err)
	}

	return nil
}

func (r *RepackUtil) getWindowsDirectories(rootPath string) (map[string]string, error) {
	dirs := map[string]string{}
	var err error
	dirs["inf"], err = shared.FindFirstMatch(rootPath, "windows", "inf")
	if err != nil {
		return nil, fmt.Errorf("Failed to determine windows/inf path: %w", err)
	}

	dirs["config"], err = shared.FindFirstMatch(rootPath, "windows", "system32", "config")
	if err != nil {
		return nil, fmt.Errorf("Failed to determine windows/system32/config path: %w", err)
	}

	dirs["drivers"], err = shared.FindFirstMatch(rootPath, "windows", "system32", "drivers")
	if err != nil {
		return nil, fmt.Errorf("Failed to determine windows/system32/drivers path: %w", err)
	}

	dirs["filerepository"], err = shared.FindFirstMatch(rootPath, "windows", "system32", "driverstore", "filerepository")
	if err != nil {
		if r.windowsVersion != "2k3" && r.windowsVersion != "xp" {
			return nil, fmt.Errorf("Failed to determine windows/system32/driverstore/filerepository path: %w", err)
		}

		// Windows 2003 doesn't have a file repository.
		dirs["filerepository"] = dirs["inf"]
	}

	dirs["system32"], err = shared.FindFirstMatch(rootPath, "windows", "system32")
	if err != nil {
		return nil, fmt.Errorf("Failed to determine windows/system32 path: %w", err)
	}

	dirs["syswow64"], err = shared.FindFirstMatch(rootPath, "windows", "syswow64")
	if err != nil {
		return nil, fmt.Errorf("Failed to determine windows/syswow64 path: %w", err)
	}

	return dirs, nil
}

func (r *RepackUtil) modifyWimIndex(wimFile string, index int, name string, driverPath string) error {
	wimIndex := strconv.Itoa(index)
	wimPath := filepath.Join(r.cacheDir, "wim", wimIndex)
	wimName := filepath.Base(wimFile)
	logger := r.logger.WithFields(logrus.Fields{"wim": wimName, "idx": wimIndex + ":" + name})
	if !incus.PathExists(wimPath) {
		err := os.MkdirAll(wimPath, 0o755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", wimPath, err)
		}
	}

	success := false
	logger.Info("Mounting")
	// Mount wim file
	err := shared.RunCommand(r.ctx, nil, nil, "wimlib-imagex", "mountrw", wimFile, wimIndex, wimPath, "--allow-other")
	if err != nil {
		return fmt.Errorf("Failed to mount %q: %w", wimName, err)
	}

	defer func() {
		if !success {
			_ = shared.RunCommand(r.ctx, nil, nil, "wimlib-imagex", "unmount", wimPath)
		}
	}()

	logger.Info("Modifying")
	// Create registry entries and copy files
	err = r.InjectDrivers(wimPath, driverPath)
	if err != nil {
		return fmt.Errorf("Failed to inject drivers: %w", err)
	}

	logger.Info("Unmounting")
	err = shared.RunCommand(r.ctx, nil, nil, "wimlib-imagex", "unmount", wimPath, "--commit")
	if err != nil {
		return fmt.Errorf("Failed to unmount WIM image %q: %w", wimName, err)
	}

	success = true
	return nil
}
