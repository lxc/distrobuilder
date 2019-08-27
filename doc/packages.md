# Package management

Installing and removing packages can be done using the `packages` section.

```yaml
packages:
    manager: <string> # required
    update: <boolean>
    cleanup: <boolean>
    sets:
        - packages:
            - <string>
            - ...
          action: <string> # required
          architectures: <array> # filter
          releases: <array> # filter
          variants: <array> # filter
        - ...
    repositories:
        - name: <string>
          url: <string>
          type: <string>
          key: <string>
          architectures: <array> # filter
          releases: <array> # filter
          variants: <array> # filter
        - ...

```

The `manager` keys specifies the package manager which is to be used.
Valid package manager are:

* apk
* apt
* dnf
* egoportage (combination of portage and ego)
* equo
* opkg
* pacman
* portage
* xbps
* yum
* zypper

It's also possible to specify a custom package manager.
This is useful if the desired package manager is not supported by distrobuilder.

```yaml
packages:
    custom-manager: # required
        clean: # required
            command: <string>
            flags: <array>
        install: # required
            command: <string>
            flags: <array>
        remove: # required
            command: <string>
            flags: <array>
        refresh: # required
            command: <string>
            flags: <array>
        update: # required
            command: <string>
            flags: <array>
    flags: <array>
    ...
```

If `update` is true, the package manager will update all installed packages.

If `cleanup` is true, the package manager will run a cleanup operation which usually cleans up cached files.
This depends on the package manager though and is not supported by all.

A set contains a list of `packages`, an `action`, and optional filters.
Here, `packages` is a list of packages which are to be installed or removed.
The value of `action` must be either `install` or `remove`.

`repositories` contains a list of additional repositories which are to be added.
The `type` field is only needed if the package manager supports more than one repository manager.
The `key` field is a GPG armored keyring which might be needed for verification.
