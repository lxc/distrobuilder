# How to build images

## Plain rootfs

```shell
$ distrobuilder build-dir --help
Build plain rootfs

Usage:
  distrobuilder build-dir <filename|-> <target dir> [flags]

Flags:
  -h, --help              help for build-dir
      --keep-sources      Keep sources after build (default true)
      --sources-dir       Sources directory for distribution tarballs (default "/tmp/distrobuilder")
      --with-post-files   Run post-files actions

Global Flags:
      --cache-dir         Cache directory
      --cleanup           Clean up cache directory (default true)
      --debug             Enable debug output
      --disable-overlay   Disable the use of filesystem overlays
  -o, --options           Override options (list of key=value)
  -t, --timeout           Timeout in seconds
      --version           Print version number

```

To build a plain rootfs, run `distrobuilder build-dir`.
The command takes an image definition file and an output directory as positional arguments.
Running `build-dir` is useful if one wants to build both LXC and Incus images.
In that case one can simply run

```shell
distrobuilder build-dir def.yaml /path/to/rootfs
distrobuilder pack-lxc def.yaml /path/to/rootfs /path/to/output
distrobuilder pack-incus def.yaml /path/to/rootfs /path/to/output
```

(howto-build-lxc)=
## LXC image

```shell
$ distrobuilder build-lxc --help
Build LXC image from scratch

The compression can be set with the --compression flag. I can take one of the
following values:
  - bzip2
  - gzip
  - lzip
  - lzma
  - lzo
  - lzop
  - xz (default)
  - zstd

For supported compression methods, a compression level can be specified with
method-N, where N is an integer, e.g. gzip-9.

Usage:
  distrobuilder build-lxc <filename|-> [target dir] [--compression=COMPRESSION] [flags]

Flags:
      --compression    Type of compression to use (default "xz")
  -h, --help           help for build-lxc
      --keep-sources   Keep sources after build (default true)
      --sources-dir    Sources directory for distribution tarballs (default "/tmp/distrobuilder")

Global Flags:
      --cache-dir         Cache directory
      --cleanup           Clean up cache directory (default true)
      --debug             Enable debug output
      --disable-overlay   Disable the use of filesystem overlays
  -o, --options           Override options (list of key=value)
  -t, --timeout           Timeout in seconds
      --version           Print version number

```

Running the `build-lxc` sub-command creates a LXC image.
It outputs two files `rootfs.tar.xz` and `meta.tar.xz`.
After building the image, the rootfs will be destroyed.

The `pack-lxc` sub-command can be used to create an image from an existing rootfs.
The rootfs won't be deleted afterwards.

(howto-build-incus)=
## Incus image

```shell
$ distrobuilder build-incus --help
Build Incus image from scratch

Depending on the type, it either outputs a unified (single tarball)
or split image (tarball + squashfs or qcow2 image). The --type flag can take one of the
following values:
  - split (default)
  - unified


The compression can be set with the --compression flag. I can take one of the
following values:
  - bzip2
  - gzip
  - lzip
  - lzma
  - lzo
  - lzop
  - xz (default)
  - zstd

For supported compression methods, a compression level can be specified with
method-N, where N is an integer, e.g. gzip-9.

Usage:
  distrobuilder build-incus <filename|-> [target dir] [--type=TYPE] [--compression=COMPRESSION] [--import-into-incus] [flags]

Flags:
      --compression               Type of compression to use (default "xz")
  -h, --help                      help for build-incus
      --import-into-incus[="-"]   Import built image into Incus
      --keep-sources              Keep sources after build (default true)
      --sources-dir               Sources directory for distribution tarballs (default "/tmp/distrobuilder")
      --type                      Type of tarball to create (default "split")
      --vm                        Create a qcow2 image for VMs

Global Flags:
      --cache-dir         Cache directory
      --cleanup           Clean up cache directory (default true)
      --debug             Enable debug output
      --disable-overlay   Disable the use of filesystem overlays
  -o, --options           Override options (list of key=value)
  -t, --timeout           Timeout in seconds
      --version           Print version number
```

Running the `build-incus` sub-command creates an Incus image.
If `--type=split`, it outputs two files.
The metadata tarball will always be named `incus.tar.xz`.
When creating a container image, the second file will be `rootfs.squashfs`.
When creating a VM image, the second file will be `disk.qcow2`.
If `--type=unified`, a unified tarball named `<image.name>.tar.xz` is created.
See the [image section](../reference/image.md) for more on the image name.

If `--compression` is set, the tarballs will use the provided compression instead of `xz`.

Setting `--vm` will create a `qcow2` image which is used for virtual machines.

If `--import-into-incus` is set, the resulting image is imported into Incus.
It basically runs `lxc image import <image>`.
Per default, it doesn't create an alias.
This can be changed by calling it as `--import-into-incus=<alias>`.

After building the image, the rootfs will be destroyed.

The `pack-incus` sub-command can be used to create an image from an existing rootfs.
The rootfs won't be deleted afterwards.
