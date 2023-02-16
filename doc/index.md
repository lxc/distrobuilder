[![LXD](../.sphinx/_static/download/containers.png)](https://linuxcontainers.org/lxd)

# `distrobuilder`

`distrobuilder` is an image building tool for LXC and LXD.

Its modern design uses pre-built official images whenever available and supports a variety of modifications on the base image.
`distrobuilder` creates LXC or LXD images, or just a plain root filesystem, from a declarative image definition (in YAML format) that defines the source of the image, its package manager, what packages to install or remove for specific image variants, OS releases and architectures, as well as additional files to generate and arbitrary actions to execute as part of the image build process.

`distrobuilder` can be used to create custom images that can be used as the base for LXC containers or LXD instances.

The LXD team uses `distrobuilder` to build the images on the [Linux containers image server](https://images.linuxcontainers.org/).
You can also use it to build images from ISOs that require licenses and therefore cannot be distributed.

---

## In this documentation

````{grid} 1 1 2 2
```{grid-item} [](tutorials/index)

**Start here**: a hands-on introduction to `distrobuilder` for new users
```
```{grid-item} [](howto/index)

**Step-by-step guides** covering key operations and common tasks
```
````

````{grid} 1 1 2 2
:reverse:

```{grid-item} [](reference/index)

**Technical information** - specifications, APIs, architecture
```

```{grid-item} Explanation (coming)

**Discussion and clarification** of key topics
```
````

---

## Project and community

`distrobuilder` is free software and developed under the [Apache 2 license](https://www.apache.org/licenses/LICENSE-2.0).
It's an open source project that warmly welcomes community projects, contributions, suggestions, fixes and constructive feedback.

The LXD project is sponsored by [Canonical Ltd](https://www.canonical.com).

- [Code of Conduct](https://github.com/lxc/lxd/blob/master/CODE_OF_CONDUCT.md) <!-- wokeignore:rule=master -->
- [Contribute to the project](https://github.com/lxc/distrobuilder/blob/master/CONTRIBUTING.md)  <!-- wokeignore:rule=master -->
- [Discuss on IRC](https://web.libera.chat/#lxc) (see [Getting started with IRC](https://discuss.linuxcontainers.org/t/getting-started-with-irc/11920) if needed)
- [Ask and answer questions on the forum](https://discuss.linuxcontainers.org)
- [Join the mailing lists](https://lists.linuxcontainers.org)


```{toctree}
:hidden:
:titlesonly:

self
tutorials/index
howto/index
reference/index
```
