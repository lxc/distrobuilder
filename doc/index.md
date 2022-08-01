# distrobuilder
System container image builder for LXC and LXD

## Status
Type            | Service               | Status
---             | ---                   | ---
CI              | GitHub                | [![Build Status](https://github.com/lxc/distrobuilder/workflows/CI%20tests/badge.svg)](https://github.com/lxc/distrobuilder/actions)
Project status  | CII Best Practices    | [![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1728/badge)](https://bestpractices.coreinfrastructure.org/projects/1728)


## Command line options

The following are the command line options of `distrobuilder`. You can use `distrobuilder` to create container images for both LXC and LXD.

```bash
$ distrobuilder
System container image builder for LXC and LXD

Usage:
  distrobuilder [command]

Available Commands:
  build-dir      Build plain rootfs
  build-lxc      Build LXC image from scratch
  build-lxd      Build LXD image from scratch
  help           Help about any command
  pack-lxc       Create LXC image from existing rootfs
  pack-lxd       Create LXD image from existing rootfs
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

## Installing from package

`distrobuilder` is available from the snapstore.

```
sudo snap install distrobuilder --classic
```

## Installing from source

To compile `distrobuilder` from source, first install the Go programming language, and some other dependencies.

- Debian-based:
    ```
    sudo apt update
    sudo apt install -y golang-go debootstrap rsync gpg squashfs-tools git
    ```
- ArchLinux-based:
    ```
    sudo pacman -Syu
    sudo pacman -S go debootstrap rsync gnupg squashfs-tools git --needed
    ```

Second, download the source code of the `distrobuilder` repository (this repository).

```
git clone https://github.com/lxc/distrobuilder
```

Third, enter the directory with the source code of `distrobuilder` and run `make` to compile the source code. This will generate the executable program `distrobuilder`, and it will be located at `$HOME/go/bin/distrobuilder`.

```
cd ./distrobuilder
make
```

Finally, you can run `distrobuilder` as follows. You may also add to your $PATH the directory `$HOME/go/bin/` so that you do not need to run the command with the full path.

```
$HOME/go/bin/distrobuilder
```

## How to use

In the following, we see how to create a container image for LXD.

### Creating a container image

To create a container image, first create a directory where you will be placing the container images, and enter that directory.

```
mkdir -p $HOME/ContainerImages/ubuntu/
cd $HOME/ContainerImages/ubuntu/
```

Then, copy one of the example yaml configuration files for container images into this directory. In this example, we are creating an Ubuntu container image.

```
cp $HOME/go/src/github.com/lxc/distrobuilder/doc/examples/ubuntu.yaml ubuntu.yaml
```

### Build the container image for LXD

Finally, run `distrobuilder` to create the container image. We are using the `build-lxd` option to create a container image for LXD.

```
sudo $HOME/go/bin/distrobuilder build-lxd ubuntu.yaml
```

If the command is successful, you will get an output similar to the following. The `lxd.tar.xz` file is the description of the container image. The `rootfs.squasfs` file is the root filesystem (rootfs) of the container image. The set of these two files is the _container image_.

```bash
$ ls -l
total 100960
-rw-r--r-- 1 root   root         676 Oct  3 16:15 lxd.tar.xz
-rw-r--r-- 1 root   root   103370752 Oct  3 16:15 rootfs.squashfs
-rw-r--r-- 1 ubuntu ubuntu      7449 Oct  3 16:03 ubuntu.yaml
$
```

### Adding the container image to LXD

To add the container image to a LXD installation, use the `lxc image import` command as follows.

```bash
$ lxc image import lxd.tar.xz rootfs.squashfs --alias mycontainerimage
Image imported with fingerprint: 009349195858651a0f883de804e64eb82e0ac8c0bc51880
```

Let's see the container image in LXD. The `ubuntu.yaml` had a setting to create an Ubuntu 20.04 (`focal`) image. The size is 98.58MB.

```bash
$ lxc image list mycontainerimage
+------------------+--------------+--------+--------------+--------+---------+-----------------------------+
|      ALIAS       | FINGERPRINT  | PUBLIC | DESCRIPTION  |  ARCH  |  SIZE   |         UPLOAD DATE         |
+------------------+--------------+--------+--------------+--------+---------+-----------------------------+
| mycontainerimage | 009349195858 | no     | Ubuntu focal | x86_64 | 98.58MB | Oct 3, 2020 at 5:10pm (UTC) |
+------------------+--------------+--------+--------------+--------+---------+-----------------------------+
```

### Launching a LXD container from the container image

To launch a container from the freshly created container image, use `lxc launch` as follows. Note that you do not specify a repository of container images (like `ubuntu:` or `images:`) because the image is located locally.

```bash
$ lxc launch mycontainerimage c1
Creating c1
Starting c1
```

### Build a LXC container image

Using LXC containers instead of LXD may require the installation of `lxc-utils`.
Having both LXC and LXD installed on the same system will probably cause confusion.
Use of raw LXC is generally discouraged due to the lack of automatic Apparmor
protection.

For LXC, instead use:
```bash
$ sudo $HOME/go/bin/distrobuilder build-lxc ubuntu.yaml
$ ls -l
total 87340
-rw-r--r-- 1 root root      740 Jan 19 03:15 meta.tar.xz
-rw-r--r-- 1 root root 89421136 Jan 19 03:15 rootfs.tar.xz
-rw-r--r-- 1 root root     4798 Jan 19 02:42 ubuntu.yaml
```

### Adding the container image to LXC

To add the container image to a LXC installation, use the `lxc-create` command as follows.

```bash
lxc-create -n myContainerImage -t local -- --metadata meta.tar.xz --fstree rootfs.tar.xz
```

Then start the container with
```bash
lxc-start -n myContainerImage
```

### Repack Windows ISO

With LXD it's possible to run Windows VMs. All you need is a Windows ISO and a bunch of drivers.
To make the installation a bit easier, `distrobuilder` added the `repack-windows` command. It takes a Windows ISO, and repacks it together with the necessary drivers.

Currently, `distrobuilder` supports Windows 10, Windows Server 2012, Windows Server 2016, Windows Server 2019 and Windows Server 2022. The Windows version will automatically be detected, but in case this fails you can use the `--windows-version` flag to set it manually. It supports the values `w10`, `2k12`, `2k16`, `2k19` and `2k22` for Windows 10, Windows Server 2012, Windows Server 2016, Windows Server 2019 and Windows Server 2022 respectively.

Here's how to repack a Windows ISO:
```bash
distrobuilder repack-windows path/to/Windows.iso path/to/Windows-repacked.iso
```

More information on `repack-windows` can be found by running

```bash
distrobuilder repack-windows -h
```

### Install Windows

Run the following commands to initialize the VM, to configure (=increase) the allocated disk space and finally attach the full path of your prepared ISO file. Note that the installation of Windows 10 takes about 10GB (before updates), therefore a 30GB disk gives you about 20GB of free space.

```bash
lxc init win10 --empty --vm -c security.secureboot=false
lxc config device override win10 root size=30GiB
lxc config device add win10 iso disk source=/path/to/Windows-repacked.iso boot.priority=10
```

Now, the VM win10 has been configured and it is ready to be started. The following command starts the virtual machine and opens up a VGA console so that we go through the graphical installation of Windows.

```bash
lxc start win10 --console=vga
```

Taken from: https://blog.simos.info/how-to-run-a-windows-virtual-machine-on-lxd-on-linux/

### Examples

Examples of yaml files for various distributions can be found in the [examples directory](./doc/examples) and in the [lxc-ci repository](https://github.com/lxc/lxc-ci/tree/master/images).

### Troubleshooting

#### Error "Cannot install into target '/var/cache/distrobuilder.123456789/rootfs' mounted with noexec or nodev"

You have installed `distrobuilder` into a LXD container and you are trying to run it. `distrobuilder` does not run in a LXD container. Run `distrobuilder` on the host, or in a VM.

##### Error "error: This revision of snap "distrobuilder" was published using classic confinement"

You are trying to install the `distrobuilder` snap package. The `distrobuilder` snap package has been configured to use the `classic` confinement. Therefore, when you install it, you have to add the flag `--classic` as shown above in the instructions.

##### Error "You must be root to run this tool"

You must be _root_ in order to run the `distrobuilder` tool. The tool runs commands such as `mknod` that require administrative privileges. Prepend `sudo` when running `distrobuilder`.
