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
          flags: <array> # install/remove flags for just this set
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

* `apk`
* `apt`
* `dnf`
* `egoportage` (combination of `portage` and `ego`)
* `equo`
* `luet`
* `opkg`
* `pacman`
* `portage`
* `slackpkg`
* `xbps`
* `yum`
* `zypper`

It's also possible to specify a custom package manager.
This is useful if the desired package manager is not supported by distrobuilder.

```yaml
packages:
    custom_manager: # required
        clean: # required
            cmd: <string>
            flags: <array>
        install: # required
            cmd: <string>
            flags: <array>
        remove: # required
            cmd: <string>
            flags: <array>
        refresh: # required
            cmd: <string>
            flags: <array>
        update: # required
            cmd: <string>
            flags: <array>
        flags: <array> # global flags for all commands
    ...
```

If `update` is true, the package manager will update all installed packages.

If `cleanup` is true, the package manager will run a cleanup operation which usually cleans up cached files.
This depends on the package manager though and is not supported by all.

A set contains a list of `packages`, an `action`, and optional filters.
Here, `packages` is a list of packages which are to be installed or removed.
The value of `action` must be either `install` or `remove`. If `flags` is
specified for a package set, they are appended to the command specific
flags, along with any global flags, when calling the `install` or `remove`
command.  For example, you can define a package set that should be installed
with `--no-install-recommends`.

`repositories` contains a list of additional repositories which are to be added.
The `type` field is only needed if the package manager supports more than one repository manager.
The `key` field is a GPG armored key ring which might be needed for verification.

Depending on the package manager, the `url` field can take the content of a repository file. The following is possible with `yum`:

```yaml
packages:
  manager: yum
  update: false
  repositories:
    - name: myrepo
      url: |-
        [myrepo]
        baseurl=http://user:password@1.1.1.1
        gpgcheck=0
```
