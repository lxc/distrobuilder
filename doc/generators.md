# Generators

Generators are used to create, modify or remove files inside the rootfs.
Available generators are

* [cloud-init](#cloud-init)
* [dump](#dump)
* [hostname](#hostname)
* [hosts](#hosts)
* [remove](#remove)
* [template](#template)
* [upstart_tty](#upstart_tty)

In the image definition yaml, they are listed under `files`.

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
      architectures: <array> # filter
      releases: <array> # filter
      variants: <array> # filter
```

Filters can be applied to each entry in `files`.
Valid filters are `architecture`, `release` and `variant`.
See filters for more information.

## cloud-init

For LXC images, the generator disables cloud-init by disabling any cloud-init services, and creates the file `cloud-init.disable` which is checked by `cloud-init` on startup.

For LXD images, the generator creates templates depending on the provided name.
Valid names are `user-data`, `meta-data`, `vendor-data` and `network-config`.
The default `path` if not defined otherwise is `/var/lib/cloud/seed/nocloud-net/<name>`.
Setting `path`, `content` or `template.properties` will override the default values.

## dump

The `dump` generator writes the provided `content` to a file set in `path`.

## hostname

For LXC images, the hostname generator writes the LXC specific string `LXC_NAME` to the hostname file set in `path`.
If the path doesn't exist, the generator does nothing.

For LXD images, the generator creates a template for `path`.
If the path doesn't exist, the generator does nothing.

## hosts

For LXC images, the generator adds the entry `127.0.0.1 LXC_NAME` to the hosts file set in `path`.

For LXD images, the generator creates a template for the hosts file set in `path`, adding an entry for `127.0.0.1 {{ container.name }}`.

## remove

The generator removes the file set in `path` from the container's root filesystem.

## template

This generator creates a custom LXD template.
The `name` field is used as the template's file name.
The `path` defines the target file in the container's root filesystem.
The `properties` key is a map of the template properties.

The `when` key can be one or more of:

* create (run at the time a new container is created from the image)
* copy (run when a container is created from an existing one)
* start (run every time the container is started)

See [LXD image format](https://lxd.readthedocs.io/en/latest/image-handling/#image-format) for more.

## upstart_tty

This generator creates an upstart job which prevents certain TTYs from starting.
The job script is written to `path`.
