# distrobuilder
System container image builder for LXC and LXD

## Status
Type            | Service               | Status
---             | ---                   | ---
CI              | Jenkins               | [![Build Status](https://travis-ci.org/lxc/distrobuilder.svg?branch=master)](https://travis-ci.org/lxc/distrobuilder)
Project status  | CII Best Practices    | [![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1728/badge)](https://bestpractices.coreinfrastructure.org/projects/1728)

## Example usage

Save one of the [Ubuntu yaml snippets](./doc/examples/ubuntu.md) as
`ubuntu.yaml`. To create a simple `Ubuntu` rootfs in a folder called
`ubuntu-rootfs` call `distrobuilder` as `distrobuilder build-dir ubuntu.yaml
ubuntu-rootfs`.
