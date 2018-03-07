# distrobuilder
System container image builder for LXC and LXD

## Example yaml file

```yaml
image:
  distribution: ubuntu # required
  release: artful # optional
  variant: default # optional
  description: Ubuntu Artful # optional
  expiry: 30d # optional: defaults to 30d
  arch: x86_64 # optional: defaults to local architecture

source:
  downloader: ubuntu-http
  url: http://cdimage.ubuntu.com/ubuntu-base
  keys:
    - 0xCODE
  keyserver: hkps.pool.sks-keyservers.net # optional

targets:
  lxc:
    create-message: |
        You just created an Ubuntu container (release=artful, arch=amd64, variant=default)

        To enable sshd, run: apt-get install openssh-server

        For security reason, container images ship without user accounts
        and without a root password.

        Use lxc-attach or chroot directly into the rootfs to set a root password
        or create user accounts.
    config: |
        lxc.include = LXC_TEMPLATE_CONFIG/ubuntu.common.conf
        lxc.arch = x86_64
    config-user: |
        lxc.include = LXC_TEMPLATE_CONFIG/ubuntu.common.conf
        lxc.include = LXC_TEMPLATE_CONFIG/ubuntu.userns.conf
        lxc.arch = x86_64

files:
 # lxc: Puts the LXC_NAME placeholder in place
 # lxd: Adds a template to generate the file on create and copy
 - path: /etc/hostname
   generator: hostname

 # lxc: Puts the LXC_NAME placeholder in place
 # lxd: Adds a template to generate the file on create
 - path: /etc/hosts
   generator: hosts

 # all: Add the upstart job to deal with ttys
 - path: /etc/init/lxc-tty.conf
   generator: upstart-tty
   releases:
    - precise
    - trusty

packages:
    manager: apt

    update: false
    install:
        - systemd
        - nginx
        - vim
    remove:
        - vim

actions:
    post-unpack: |-
      #!/bin/sh
      echo "This is run after unpacking the downloaded content"

    post-update: |-
      #!/bin/sh
      echo "This is run after updating all packages"

    post-packages: |-
      #!/bin/sh
      echo "This is run after installing/removing packages"

    post-files: |-
      #!/bin/sh
      echo "This is run after running the file templates"

mappings:
    architecture_map: debian
```
