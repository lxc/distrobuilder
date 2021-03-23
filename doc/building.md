# Building images

## Plain rootfs

```shell
$ distrobuilder build-dir --help
Build plain rootfs

Usage:
  distrobuilder build-dir <filename|-> <target dir> [flags]

Flags:
  -h, --help   help for build-dir

Global Flags:
      --cache-dir   Cache directory
      --cleanup     Clean up cache directory (default true)
  -o, --options     Override options (list of key=value)
  -t, --timeout     Timeout in seconds
```

To build a plain rootfs, run `distrobuilder build-dir`.
The command takes an image definition file and an output directory as positional arguments.
Running `build-dir` is useful if one wants to build both LXC and LXD images.
In that case one can simply run

```shell
distrobuilder build-dir def.yaml /path/to/rootfs
distrobuilder pack-lxc def.yaml /path/to/rootfs /path/to/output
distrobuilder pack-lxd def.yaml /path/to/rootfs /path/to/output
```

## LXC image

```shell
$ distrobuilder build-lxc --help
Build LXC image from scratch

Usage:
  distrobuilder build-lxc <filename|-> [target dir] [flags]

Flags:
  -h, --help   help for build-lxc

Global Flags:
      --cache-dir   Cache directory
      --cleanup     Clean up cache directory (default true)
  -o, --options     Override options (list of key=value)
  -t, --timeout     Timeout in seconds
```

Running the `build-lxc` subcommand creates a LXC image.
It outputs two files `rootfs.tar.xz` and `meta.tar.xz`.
After building the image, the rootfs will be destroyed.

The `pack-lxc` subcommand can be used to create an image from an existing rootfs.
The rootfs won't be deleted afterwards.

## LXD image

```shell
$ distrobuilder build-lxd --help
Build LXD image from scratch

Usage:
  distrobuilder build-lxd <filename|-> [target dir] [--type=TYPE] [--compression=COMPRESSION] [flags]

Flags:
      --compression   Type of compression to use (default "xz")
  -h, --help          help for build-lxd
      --type          Type of tarball to create (default "split")
      --vm            Create a qcow2 image for VMs

Global Flags:
      --cache-dir   Cache directory
      --cleanup     Clean up cache directory (default true)
  -o, --options     Override options (list of key=value)
  -t, --timeout     Timeout in seconds
```

Running the `build-lxd` subcommand creates a LXD image.
If `--type=split`, it outputs two files.
The metadata tarball will always be named `lxd.tar.xz`.
When creating a container image, the second file will be `rootfs.squashfs`.
When creating a VM image, the second file will be `disk.qcow2`.
If `--type=unified`, a unified tarball named `<image.name>.tar.xz` is created.
See the [image section](image.md) for more on the image name.

If `--compression` is set, the tarballs will use the provided compression instead of `xz`.

Setting `--vm` will create a qcow2 image which is used for virtual machines.

After building the image, the rootfs will be destroyed.

The `pack-lxd` subcommand can be used to create an image from an existing rootfs.
The rootfs won't be deleted afterwards.
