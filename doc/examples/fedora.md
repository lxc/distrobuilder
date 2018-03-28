# Fedora

```yaml
image:
  distribution: fedora
  release: 28
  description: Fedora {{ image.release }}
  expiry: 30d
  arch: x86_64

source:
  downloader: fedora-http
  url: https://kojipkgs.fedoraproject.org

targets:
  lxc:
    create-message: |
      You just created a Fedora container (release={{ image.release }}, arch={{ image.architecture }})

    config:
      - type: all
        before: 5
        content: |-
          lxc.include = LXC_TEMPLATE_CONFIG/fedora.common.conf

      - type: user
        before: 5
        content: |-
          lxc.include = LXC_TEMPLATE_CONFIG/fedora.userns.conf

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
  - path: /etc/hostname
    generator: hostname

  - path: /etc/hosts
    generator: hosts

packages:
  manager: dnf

  update: true
  install:
    - systemd
    - neovim
```
