# Mappings

`mappings` describes an architecture mapping between the architectures from those used in LXD and those used by the distribution.
These mappings are useful if you for example want to build a `x86_64` image but the source tarball contains `amd64` as its architecture.

```yaml
mappings:
    architectures: <map>
    architecture_map: <string>
```

It's possible to specify a custom map using the `architectures` field.
Heres an example of a custom mapping:

```yaml
mappings:
    architectures:
        i686: i386
        x86_64: amd64
        armv7l: armhf
        aarch64: arm64
        ppc: powerpc
        ppc64: powerpc64
        ppc64le: ppc64el
```

The mapped architecture can be accessed via `Image.ArchitectureMapped` in the code or `image.architecture_mapped` in the definition file.

There are some preset mappings which can be used in the `architecture_map` field.
Those are:

* alpinelinux
* altlinux
* archlinux
* centos
* debian
* funtoo
* gentoo
* plamolinux
* voidlinux
