# distrobuilder
System container image builder for LXC and LXD

## Status
Type            | Service               | Status
---             | ---                   | ---
CI              | Jenkins               | [![Build Status](https://travis-ci.org/lxc/distrobuilder.svg?branch=master)](https://travis-ci.org/lxc/distrobuilder)
Project status  | CII Best Practices    | [![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1728/badge)](https://bestpractices.coreinfrastructure.org/projects/1728)


## Command line options

The following are the command line options of `distrobuilder`. You can use `distrobuilder` to create container images for both LXC and LXD.

```bash
$ distrobuilder
System container image builder for LXC and LXD

Usage:
  distrobuilder [command]

Available Commands:
  build-dir   Build plain rootfs
  build-lxc   Build LXC image from scratch
  build-lxd   Build LXD image from scratch
  help        Help about any command
  pack-lxc    Create LXC image from existing rootfs
  pack-lxd    Create LXD image from existing rootfs

Flags:
      --cache-dir   Cache directory
      --cleanup     Clean up cache directory (default true)
  -h, --help        help for distrobuilder
  -o, --options     Override options (list of key=value)

Use "distrobuilder [command] --help" for more information about a command.
```

## How to use

In the following, we see how to create a container image for LXD.

## Installing from package

`distrobuilder` is available from the snapstore.

```
snap install distrobuilder --classic
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

Second, download the source code of the `distrobuilder` repository (this repository). The source will be placed in `$HOME/go/src/github.com/lxc/distrobuilder/`

```
go get -d -v github.com/lxc/distrobuilder/distrobuilder
```

Third, enter the directory with the source code of `distrobuilder` and run `make` to compile the source code. This will generate the executable program `distrobuilder`, and it will be located at `$HOME/go/bin/distrobuilder`.

```
cd $HOME/go/src/github.com/lxc/distrobuilder
make
cd
```

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
total 121032
-rw-r--r-- 1 root      root            560 Oct  3 13:28 lxd.tar.xz
-rw-r--r-- 1 root      root      123928576 Oct  3 13:28 rootfs.squashfs
-rw-rw-r-- 1 multipass multipass      3317 Oct  3 13:19 ubuntu.yaml
$
```

### Adding the container image to LXD

To add the container image to a LXD installation, use the `lxc image import` command as follows.

```bash
$ lxc image import lxd.tar.xz rootfs.squashfs --alias mycontainerimage 
Image imported with fingerprint: ae81c04327b5b115383a4f90b969c97f5ef417e02d4210d40cbb17a038729a27
```

Let's see the container image in LXD. The `ubuntu.yaml` had a setting to create an Ubuntu 17.10 (`artful`) image. The size is 118MB.

```bash
$ lxc image list mycontainerimage
+------------------+--------------+--------+---------------+--------+----------+------------------------------+
|      ALIAS       | FINGERPRINT  | PUBLIC |  DESCRIPTION  |  ARCH  |   SIZE   |         UPLOAD DATE          |
+------------------+--------------+--------+---------------+--------+----------+------------------------------+
| mycontainerimage | ae81c04327b5 | no     | Ubuntu artful | x86_64 | 118.19MB | Oct 3, 2018 at 12:09pm (UTC) |
+------------------+--------------+--------+---------------+--------+----------+------------------------------+
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

### Examples

Examples of yaml files for various distributions can be found in the [examples directory](./doc/examples) and in the [lxc-ci repository](https://github.com/lxc/lxc-ci/tree/master/images).
