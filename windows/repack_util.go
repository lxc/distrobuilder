package windows

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v4"
	"github.com/lxc/incus/v6/shared/subprocess"
	incus "github.com/lxc/incus/v6/shared/util"
	"github.com/sirupsen/logrus"

	"github.com/lxc/distrobuilder/shared"
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
				if err = shared.Copy(sourcePath, targetPath); err != nil {
					return err
				}
			}
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

		for _, c := range driverInfo.SystemCatalog {
			if c.Content == "" || c.Path == "" || c.CtxKey == "" || c.ID == "" {
				continue
			}

			// Use unsafe-printable-strings so we can read the field corresponding to c.ID.
			reg, err := subprocess.RunCommand("hivexregedit", "--export", `--prefix=HKLM\SYSTEM`, "--unsafe-printable-strings", filepath.Join(dirs["config"], "SYSTEM"), c.Path)
			if err != nil {
				return fmt.Errorf("Failed to read system catalog %q for driver %q: %w", c.Path, driverName, err)
			}

			sc := bufio.NewScanner(strings.NewReader(reg))
			var matchedKey bool
			var lineExists bool
			index := CatalogIndex{}
			for sc.Scan() {
				line := sc.Text()
				if line == "" {
					continue
				} else if line == `[HKLM\SYSTEM\`+c.Path+"]" {
					// We found the key.
					matchedKey = true
					continue
				} else if strings.HasPrefix(line, "[") {
					// We found some other key.
					matchedKey = false
				} else if matchedKey {
					key, val, ok := strings.Cut(line, ":")
					if !ok || key == "" || val == "" {
						return fmt.Errorf("Invalid catalog index record %q for %q", line, driverName)
					}

					valInt, err := strconv.ParseInt(val, 16, 64)
					if err != nil {
						return fmt.Errorf("Failed to parse catalog index record %q for %q: %w", line, driverName, err)
					}

					switch key {
					case `"Next_Catalog_Entry_ID"=dword`:
						index.NextEntryID = int(valInt) + 1
					case `"Num_Catalog_Entries"=dword`:
						index.NumEntries = int(valInt) + 1
					case `"Num_Catalog_Entries64"=dword`:
						index.NumEntries64 = int(valInt) + 1
					case `"Serial_Access_Num"=dword`:
						index.SerialAccessNum = int(valInt) + 1
					}
				} else if line == c.ID {
					// Another entry exists already.
					lineExists = true
					break
				}
			}

			if lineExists {
				logger.WithFields(logrus.Fields{"hivefile": "SYSTEM", "catalog": c.ID, "path": c.Path}).Debug("Catalog entry with ID already exists")
				continue
			}

			if index.NextEntryID == 0 || index.NumEntries == 0 || index.NumEntries64 == 0 || index.SerialAccessNum == 0 {
				return fmt.Errorf("Incomplete catalog index %+v for %q", index, driverName)
			}

			ctx[c.CtxKey] = map[string]string{
				"nextID":       fmt.Sprintf("%08x", index.NextEntryID),
				"numEntries":   fmt.Sprintf("%08x", index.NumEntries),
				"numEntries64": fmt.Sprintf("%08x", index.NumEntries64),
				"serialNum":    fmt.Sprintf("%08x", index.SerialAccessNum),
				"pathIndex":    fmt.Sprintf("%012d", index.NumEntries),
				"pathIndex64":  fmt.Sprintf("%012d", index.NumEntries64),
			}

			tpl, err := pongo2.FromString(c.Content)
			if err != nil {
				return fmt.Errorf("Failed to parse %q catalog template for driver %q: %w", c.ID, driverName, err)
			}

			out, err := tpl.Execute(ctx)
			if err != nil {
				return fmt.Errorf("Failed to render %q catalog template for driver %q: %w", c.ID, driverName, err)
			}

			systemRegistry = fmt.Sprintf("%s\n\n%s", systemRegistry, out)
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
	}

	logger.WithField("hivefile", "DRIVERS").Debug("Updating Windows registry")

	err = shared.RunCommand(r.ctx, strings.NewReader(driversRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\DRIVERS'", filepath.Join(dirs["config"], "DRIVERS"))
	if err != nil {
		return fmt.Errorf("Failed to edit Windows DRIVERS registry: %w", err)
	}

	logger.WithField("hivefile", "SYSTEM").Debug("Updating Windows registry")

	err = shared.RunCommand(r.ctx, strings.NewReader(systemRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\SYSTEM'", filepath.Join(dirs["config"], "SYSTEM"))
	if err != nil {
		return fmt.Errorf("Failed to edit Windows SYSTEM registry: %w", err)
	}

	logger.WithField("hivefile", "SOFTWARE").Debug("Updating Windows registry")

	err = shared.RunCommand(r.ctx, strings.NewReader(softwareRegistry), nil, "hivexregedit", "--merge", "--prefix='HKEY_LOCAL_MACHINE\\SOFTWARE'", filepath.Join(dirs["config"], "SOFTWARE"))
	if err != nil {
		return fmt.Errorf("Failed to edit Windows SOFTWARE registry: %w", err)
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
		return nil, fmt.Errorf("Failed to determine windows/system32/driverstore/filerepository path: %w", err)
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
