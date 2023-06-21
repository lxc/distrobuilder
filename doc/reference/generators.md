# Generators

Generators are used to create, modify or remove files inside the rootfs.
Available generators are

* [`cloud-init`](#cloud-init)
* [`dump`](#dump)
* [`copy`](#copy)
* [`hostname`](#hostname)
* [`hosts`](#hosts)
* [`remove`](#remove)
* [`template`](#template)
* [`lxd-agent`](#lxd-agent)
* [`fstab`](#fstab)

In the image definition YAML, they are listed under `files`.

```yaml
files:
    - generator: <string> # which generator to use (required)
      name: <string>
      path: <string>
      content: <string>
      template:
          properties: <map>
          when: <array>
      templated: <boolean>
      mode: <string>
      gid: <string>
      uid: <string>
      pongo: <boolean>
      source: <string>
      architectures: <array> # filter
      releases: <array> # filter
      variants: <array> # filter
```

Filters can be applied to each entry in `files`.
Valid filters are `architecture`, `release` and `variant`.
See filters for more information.

If `pongo` is `true`, the values of `path`, `content`, and `source` are rendered using Pongo2.

## `cloud-init`

For LXC images, the generator disables cloud-init by disabling any cloud-init services, and creates the file `cloud-init.disable` which is checked by `cloud-init` on startup.

For LXD images, the generator creates templates depending on the provided name.
Valid names are `user-data`, `meta-data`, `vendor-data` and `network-config`.
The default `path` if not defined otherwise is `/var/lib/cloud/seed/nocloud-net/<name>`.
Setting `path`, `content` or `template.properties` will override the default values.

## `dump`

The `dump` generator writes the provided `content` to a file set in `path`.
If provided, it will set the `mode` (octal format), `gid` (integer) and/or `uid` (integer).

## `copy`

The `copy` generator copies the file(s) from `source` to the destination `path`.
`path` can be left empty and in that case the data will be placed in the same `source` path but inside the container.
If provided, the destination `path` will set the `mode` (octal format), `gid` (integer) and/or `uid` (integer).
Copying will be done according to the following rules:

* If `source` is a directory, the entire contents of the directory are copied. Only symlinks and regular files are supported.

   * Note 1: The directory itself is not copied, just its contents.
   * Note 2: For files copied, only regular Unix permissions are kept.

* If `source` is a symlink or a regular file, it is copied individually along with its metadata.
  In this case, if `path` ends with a trailing slash `/`, it will be considered a directory and the contents of `source` will be written at `path`/base(`source`).
* If `path` does not end with a trailing slash, it will be considered a regular file and the contents of `source` will be written at `path`.
* If `path` does not exist, it is created along with all missing directories in its path.
* Multiple `source` resources can be specified using Golang `filepath.Match` regexps.
  For simplicity they are only allowed in the base name and not in the directory hierarchy.
  If more than one match is found, `path` will be automatically interpreted as a directory.

## `hostname`

For LXC images, the host name generator writes the LXC specific string `LXC_NAME` to the `hostname` file set in `path`.
If the path doesn't exist, the generator does nothing.

For LXD images, the generator creates a template for `path`.
If the path doesn't exist, the generator does nothing.

## `hosts`

For LXC images, the generator adds the entry `127.0.0.1 LXC_NAME` to the hosts file set in `path`.

For LXD images, the generator creates a template for the hosts file set in `path`, adding an entry for `127.0.0.1 {{ container.name }}`.

## `remove`

The generator removes the file set in `path` from the container's root file system.

## `template`

This generator creates a custom LXD template.
The `name` field is used as the template's file name.
The `path` defines the target file in the container's root file system.
The `properties` key is a map of the template properties.

The `when` key can be one or more of:

* create (run at the time a new container is created from the image)
* copy (run when a container is created from an existing one)
* start (run every time the container is started)

See {ref}`lxd:image-format` in the LXD documentation for more information.

## `lxd-agent`

This generator creates the `systemd` unit files which are needed to start the `lxd-agent` in LXD VMs.

## `fstab`

This generator creates an `/etc/fstab` file which is used for VMs.
Its content is:

```
LABEL=rootfs  /         <fs>  <options>  0 0
LABEL=UEFI    /boot/efi vfat  defaults   0 0
```

The file system is taken from the LXD target (see [targets](targets.md)) which defaults to `ext4`.
The options are generated depending on the file system.
You cannot override them.
