# Use `distrobuilder` to create images

This guide shows you how to create an image for Incus or LXC.

Before you start, you must install `distrobuilder`.
See {doc}`../howto/install` for instructions.

## Create an image

To create an image, first create a directory where you will be placing the images, and enter that directory.

```
mkdir -p $HOME/Images/ubuntu/
cd $HOME/Images/ubuntu/
```

Then, copy one of the example YAML configuration files for images into this directory.

```{note}
The YAML configuration file contains an image template that gives instructions to distrobuilder.

Distrobuilder provides examples of YAML files for various distributions in the [examples directory](https://github.com/lxc/distrobuilder/tree/master/doc/examples).
[`scheme.yaml`](https://github.com/lxc/distrobuilder/blob/master/doc/examples/scheme.yaml) is a standard template that includes all available options.

Official Incus templates for various distributions are available in the [`lxc-ci` repository](https://github.com/lxc/lxc-ci/tree/master/images).
```

In this example, we are creating an Ubuntu image.

```
cp $HOME/go/src/github.com/lxc/distrobuilder/doc/examples/ubuntu.yaml ubuntu.yaml
```

### Edit the template file

Optionally, you can do some edits to the YAML configuration file.
You can define the following keys:

| Section    | Description                                                                              | Documentation                  |
|------------|------------------------------------------------------------------------------------------|--------------------------------|
| `image`    | Defines distribution, architecture, release etc.                                         | {doc}`../reference/image`      |
| `source`   | Defines main package source, keys etc.                                                   | {doc}`../reference/source`     |
| `targets`  | Defines configuration for specific targets (e.g. Incus, instances etc.)                  | {doc}`../reference/targets`    |
| `files`    | Defines generators to modify files                                                       | {doc}`../reference/generators` |
| `packages` | Defines packages for install or removal; adds repositories                               | {doc}`../reference/packages`   |
| `actions`  | Defines scripts to be run after specific steps during image building                     | {doc}`../reference/actions`    |
| `mappings` | Maps different terms for architectures for specific distributions (e.g. `x86_64: amd64`) | {doc}`../reference/mappings`   |

```{tip}
When building a VM image, you should either build an image with cloud-init support (provides automatic size growth) or set a higher size in the template, because the standard size is relatively small (~4 GB). Alternatively, you can also grow it manually.
```

## Build and launch the image

The steps for building and launching the image depend on whether you want to use it with Incus or with LXC.

### Create an image for Incus

To build an image for Incus, run `distrobuilder`. We are using the `build-incus` option to create an image for Incus.

- To create a container image:

  ```
  sudo $HOME/go/bin/distrobuilder build-incus ubuntu.yaml
  ```

- To create a VM image:

  ```
  sudo $HOME/go/bin/distrobuilder build-incus ubuntu.yaml --vm
  ```

See {ref}`howto-build-incus` for more information about the `build-incus` command.

If the command is successful, you will get an output similar to the following (for a container image). The `incus.tar.xz` file is the description of the container image. The `rootfs.squasfs` file is the root file system (rootfs) of the container image. The set of these two files is the _container image_.

```bash
$ ls -l
total 100960
-rw-r--r-- 1 root   root         676 Oct  3 16:15 incus.tar.xz
-rw-r--r-- 1 root   root   103370752 Oct  3 16:15 rootfs.squashfs
-rw-r--r-- 1 ubuntu ubuntu      7449 Oct  3 16:03 ubuntu.yaml
$
```

#### Add the image to Incus

To add the image to an Incus installation, use the `incus image import` command as follows.

```bash
$ incus image import incus.tar.xz rootfs.squashfs --alias mycontainerimage
Image imported with fingerprint: 009349195858651a0f883de804e64eb82e0ac8c0bc51880
```

See {ref}`incus:images-copy` for detailed information.

Let's look at the image in Incus. The `ubuntu.yaml` had a setting to create an Ubuntu 20.04 (`focal`) image. The size is 98.58MB.

```bash
$ incus image list mycontainerimage
+------------------+--------------+--------+--------------+--------+---------+-----------------------------+
|      ALIAS       | FINGERPRINT  | PUBLIC | DESCRIPTION  |  ARCH  |  SIZE   |         UPLOAD DATE         |
+------------------+--------------+--------+--------------+--------+---------+-----------------------------+
| mycontainerimage | 009349195858 | no     | Ubuntu focal | x86_64 | 98.58MB | Oct 3, 2020 at 5:10pm (UTC) |
+------------------+--------------+--------+--------------+--------+---------+-----------------------------+
```

#### Launch an Incus container from the container image

To launch a container from the freshly created image, use `incus launch` as follows. Note that you do not specify a repository for the image (like `ubuntu:` or `images:`) because the image is located locally.

```bash
$ incus launch mycontainerimage c1
Creating c1
Starting c1
```

### Create an image for LXC

Using LXC containers instead of Incus may require the installation of `lxc-utils`.
Having both LXC and Incus installed on the same system will probably cause confusion.
Use of raw LXC is generally discouraged due to the lack of automatic AppArmor
protection.

To create an image for LXC, use the following command:

```bash
$ sudo $HOME/go/bin/distrobuilder build-lxc ubuntu.yaml
$ ls -l
total 87340
-rw-r--r-- 1 root root      740 Jan 19 03:15 meta.tar.xz
-rw-r--r-- 1 root root 89421136 Jan 19 03:15 rootfs.tar.xz
-rw-r--r-- 1 root root     4798 Jan 19 02:42 ubuntu.yaml
```

See {ref}`howto-build-lxc` for more information about the `build-lxc` command.

#### Add the container image to LXC

To add the container image to a LXC installation, use the `lxc-create` command as follows.

```bash
lxc-create -n myContainerImage -t local -- --metadata meta.tar.xz --fstree rootfs.tar.xz
```

#### Launch a LXC container from the container image

Then start the container with

```bash
lxc-start -n myContainerImage
```

## Repack Windows ISO

With Incus it's possible to run Windows VMs. All you need is a Windows ISO and a bunch of drivers.
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
incus init win10 --empty --vm -c security.secureboot=false
incus config device override win10 root size=30GiB
incus config device add win10 iso disk source=/path/to/Windows-repacked.iso boot.priority=10
```

Now, the VM win10 has been configured and it is ready to be started. The following command starts the virtual machine and opens up a VGA console so that we go through the graphical installation of Windows.

```bash
incus start win10 --console=vga
```
