package main

/*
#define _GNU_SOURCE
#include <errno.h>
#include <sched.h>
#include <stdio.h>
#include <string.h>
#include <sys/mount.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

__attribute__((constructor)) void init(void) {
	pid_t pid;
	int ret;

	if (geteuid() != 0) {
		fprintf(stderr, "Need to run as root\n");
		_exit(1);
	}

	// Unshare a new mntns so our mounts don't leak
	if (unshare(CLONE_NEWNS | CLONE_NEWPID) < 0) {
		fprintf(stderr, "Failed to unshare namespaces: %s\n", strerror(errno));
		_exit(1);
	}

	// Prevent mount propagation back to initial namespace
	if (mount(NULL, "/", NULL, MS_REC | MS_PRIVATE, NULL) < 0) {
		fprintf(stderr, "Failed to mark / private: %s\n", strerror(errno));
		_exit(1);
	}

	pid = fork();
	if (pid < 0) {
		fprintf(stderr, "Failed to fork: %s\n", strerror(errno));
		_exit(1);
	} else if (pid > 0) {
		// parent
		waitpid(pid, &ret, 0);
		_exit(WEXITSTATUS(ret));
	}

	// We're done, jump back to Go
}
*/
import "C"
import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/sources"

	cli "gopkg.in/urfave/cli.v1"
	yaml "gopkg.in/yaml.v2"
)

func init() {
	os.Setenv("PATH", "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin")
	os.Setenv("SHELL", "/bin/sh")
	os.Setenv("TERM", "xterm")

}

func main() {
	app := cli.NewApp()
	app.Usage = "image generator"
	// INPUT can either be a file or '-' which reads from stdin
	app.ArgsUsage = "[file|-]"
	app.HideHelp = true
	app.Action = run
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "lxc",
			Usage: "generate LXC image files",
		},
		cli.BoolFlag{
			Name:  "lxd",
			Usage: "generate LXD image files",
		},
		cli.BoolFlag{
			Name:  "plain",
			Usage: "generate plain chroot",
		},
		cli.BoolTFlag{
			Name:  "unified",
			Usage: "output unified tarball for LXD images",
		},
		cli.BoolTFlag{
			Name:  "cleanup",
			Usage: "clean up build directory",
		},
		cli.StringFlag{
			Name:  "template-dir",
			Usage: "template directory",
		},
		cli.StringFlag{
			Name:  "cache-dir",
			Usage: "cache directory",
			Value: "/var/cache/distrobuilder",
		},
		cli.StringFlag{
			Name:  "compression",
			Usage: "compression algorithm",
		},
		cli.BoolFlag{
			Name:  "help, h",
			Usage: "show help",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(c *cli.Context) error {
	var (
		def shared.Definition
		//	distro     distributions.Distribution
		downloader sources.Downloader
	)

	os.RemoveAll(c.GlobalString("cache-dir"))
	os.MkdirAll(c.GlobalString("cache-dir"), 0755)

	def, err := getDefinition(c.Args().Get(0))
	if err != nil {
		return fmt.Errorf("Error getting definition: %s", err)
	}

	downloader = sources.Get(def.Source.Downloader)
	if downloader == nil {
		return fmt.Errorf("Unsupported source downloader: %s", def.Source.Downloader)
	}

	err = downloader.Run(def.Source.URL, def.Image.Release, def.Image.Variant,
		def.Image.Arch, c.GlobalString("cache-dir"))
	if err != nil {
		return fmt.Errorf("Error while downloading source: %s", err)
	}

	if c.GlobalBoolT("cleanup") {
		defer os.RemoveAll(c.GlobalString("cache-dir"))
	}

	// enter chroot
	exitChroot, err := setupChroot(filepath.Join(c.GlobalString("cache-dir"), "rootfs"))
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %s", err)
	}

	err = managePackages(def.Packages)
	if err != nil {
		exitChroot()
		return fmt.Errorf("Failed to manage packages: %s", err)
	}

	exitChroot()

	return nil
}

func getDefinition(fname string) (shared.Definition, error) {
	var (
		err error
		buf bytes.Buffer
		def shared.Definition
	)

	if fname == "" || fname == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			buf.WriteString(scanner.Text())
		}
	} else {
		f, err := os.Open(fname)
		if err != nil {
			return def, err
		}
		defer f.Close()

		_, err = io.Copy(&buf, f)
		if err != nil {
			return def, err
		}
	}

	err = yaml.Unmarshal(buf.Bytes(), &def)
	if err != nil {
		return def, err
	}

	shared.SetDefinitionDefaults(&def)
	err = shared.ValidateDefinition(def)

	return def, err
}
