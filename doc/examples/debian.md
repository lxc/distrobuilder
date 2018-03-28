# Debian

```yaml
image:
  distribution: debian
  release: testing
  description: Debian testing
  expiry: 30d

source:
  downloader: debootstrap

targets:
  lxc:
    create-message: |
        You just created a Debian container (release={{ image.release }}, arch={{ image.architecture }}, variant={{ image.variant }})

    config:
      - type: all
        before: 5
        content: |-
          lxc.include = LXC_TEMPLATE_CONFIG/debian.common.conf

      - type: user
        before: 5
        content: |-
          lxc.include = LXC_TEMPLATE_CONFIG/debian.userns.conf

      - type: all
        after: 4
        content: |-
          lxc.include = LXC_TEMPLATE_CONFIG/common.conf

      - type: user
        after: 4
        content: |-
          lxc.include = LXC_TEMPLATE_CONFIG/userns.conf

      - type: all
        content: |-
          lxc.arch = {{ image.architecture_kernel }}

files:
 - name: hostname
   path: /etc/hostname
   generator: hostname

 - name: hosts
   path: /etc/hosts
   generator: hosts

packages:
    manager: apt

    update: true
    install:
        - systemd
        - neovim

mappings:
  architecture_map: debian
```
