# Source

In order to create an image, a source must be defined.
The source section is defined as follows:

```yaml
source:
    downloader: <string> # required
    url: <string>
    keys: <array>
    keyserver: <string>
    variant: <string>
    suite: <string>
    same_as: <boolean>
    apt_sources: <string>
    skip_verification: <boolean>
    early_packages: <array>
```

The `downloader` field defines a downloader which pulls a rootfs image which will be used as a starting point.
It needs to be one of

* alpinelinux-http
* alt-http
* apertis-http
* archlinux-http
* centos-http
* debootstrap
* docker-http
* fedora-http
* funtoo-http
* gentoo-http
* opensuse-http
* openwrt-http
* oraclelinux-http
* sabayon-http
* ubuntu-http
* voidlinux-http

The `url` field defines the URL or mirror of the rootfs image.
Although this field is not required, most downloaders will need it.

The `keys` field is a list of GPG keys.
These keys can be listed as fingerprints or armored keys.
The latter has the advantage of not having to rely on a keyserver to download the key from.
The keys are used to verify the downloaded rootfs tarball if downloaded from a insecure source (http).

The `keyserver` defaults to `hkps.pool.sks-keyservers.net` if none is provided.

The `variant` field is only used in a few distributions and defaults to `default`.
Here's a list downloaders and their possible variants:

* `centos-http`: minimal, netinstall, LiveDVD
* `debootstrap`: default, minbase, buildd, fakechroot
* `ubuntu-http`: default, core
* `voidlinux-http`: default, musl

All other downloaders ignore this field.

The `suite` field is only used by the `debootstrap` downloader.
If set, debootstrap will use `suite` instead of `image.release` as its first positional argument.

If the `same_as` field is set, distrobuilder creates a temporary symlink in `/usr/share/debootstrap/scripts` which points to the `same_as` file inside that directory.
This can be used if you want to run `debootstrap foo` but `foo` is missing due to debootstrap not being up-to-date.

If `skip_verification` is true, the source tarball is not verified.

`early_packages` is a list of packages which is to be installed while the source is being downloaded.
This is only used by the `debootstrap` downloader.
