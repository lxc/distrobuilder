# distrobuilder
System container and VM image builder for Incus and LXC.

## Status
Type            | Service               | Status
---             | ---                   | ---
CI              | GitHub                | [![Build Status](https://github.com/lxc/distrobuilder/workflows/Tests/badge.svg)](https://github.com/lxc/distrobuilder/actions)
Project status  | CII Best Practices    | [![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1728/badge)](https://bestpractices.coreinfrastructure.org/projects/1728)


## Command line options

<!-- Include start CLI -->
The following are the command line options of `distrobuilder`. You can use `distrobuilder` to create container images for both Incus and LXC.

```bash
$ distrobuilder
System container and VM image builder for Incus and LXC

Usage:
  distrobuilder [command]

Available Commands:
  build-dir      Build plain rootfs
  build-incus    Build Incus image from scratch
  build-lxc      Build LXC image from scratch
  help           Help about any command
  pack-incus     Create Incus image from existing rootfs
  pack-lxc       Create LXC image from existing rootfs
  repack-windows Repack Windows ISO with drivers included

Flags:
      --cache-dir         Cache directory
      --cleanup           Clean up cache directory (default true)
      --debug             Enable debug output
      --disable-overlay   Disable the use of filesystem overlays
  -h, --help              help for distrobuilder
  -o, --options           Override options (list of key=value)
  -t, --timeout           Timeout in seconds
      --version           Print version number

Use "distrobuilder [command] --help" for more information about a command.

```
<!-- Include end CLI -->

<!-- Include start installing -->
## Installing from package

`distrobuilder` is available from the [Snap Store](https://snapcraft.io/distrobuilder).

```
sudo snap install distrobuilder --classic
```

## Installing from source

To compile `distrobuilder` from source, first install the Go programming language, and some other dependencies.

- Debian-based:
    ```
    sudo apt update
    sudo apt install -y golang-go gcc debootstrap rsync gpg squashfs-tools git make build-essential libguestfs-tools wimtools genisoimage
    ```

- ArchLinux-based:
    ```
    sudo pacman -Syu
    sudo pacman -S go gcc debootstrap rsync gnupg squashfs-tools git make hivex cdrtools wimlib --needed
    ```

- Red Hat-based:
    ```
	sudo dnf check-update
    sudo dnf install golang gcc debootstrap rsync gnupg2 squashfs-tools git make hivex genisoimage
    ```

NOTE: Distrobuilder requires Go 1.21 or higher, if your distribution doesn't have a recent enough version available, [get it from upstream](https://go.dev/doc/install).

Second, download the source code of the `distrobuilder` repository (this repository).

```
mkdir -p $HOME/go/src/github.com/lxc/
cd $HOME/go/src/github.com/lxc/
git clone https://github.com/lxc/distrobuilder
```

Third, enter the directory with the source code of `distrobuilder` and run `make` to compile the source code. This will generate the executable program `distrobuilder`, and it will be located at `$HOME/go/bin/distrobuilder`.

```
cd ./distrobuilder
make
```

Finally, you can run `distrobuilder` as follows.
```
$HOME/go/bin/distrobuilder
```

You may also add the directory `$HOME/go/bin/` to your $PATH so that you do not need to run the command with the full path.
<!-- Include end installing -->

## How to use

See [How to use `distrobuilder`](doc/howto/build.md) for instructions.

## Troubleshooting

See [Troubleshoot `distrobuilder`](doc/howto/troubleshoot.md).
