# CentOS

```yaml
image:
  distribution: centos
  release: 7
  variant: Minimal
  description: CentOS {{ image.release }}
  expiry: 30d
  arch: x86_64

source:
  downloader: centos-http
  url: http://centos.uib.no
  keys:
    - 24C6A8A7F4A80EB5
  variant: Minimal

targets:
  lxc:
    create-message: |
        You just created a CentOS container (release={{ image.release }}, arch={{ image.architecture }})

    config:
      - type: all
        before: 5
        content: |-
          lxc.include = LXC_TEMPLATE_CONFIG/centos.common.conf

      - type: user
        before: 5
        content: |-
          lxc.include = LXC_TEMPLATE_CONFIG/centos.userns.conf

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
    manager: yum

    update: true
    install:
        - vim

actions:
  - trigger: post-unpack
    action: |-
      #!/bin/sh
      cd /mnt/cdrom/Packages
      rpm -ivh --nodeps rpm-4.11.3-25.el7.x86_64.rpm
      rpm -ivh --nodeps yum-3.4.3-154.el7.centos.noarch.rpm

      # add cdrom repo
      mkdir -p /etc/yum.repos.d
      cat <<- EOF > /etc/yum.repos.d/cdrom.repo
      [cdrom]
      name=Install CD-ROM
      baseurl=file:///mnt/cdrom
      enabled=0
      gpgcheck=1
      gpgkey=file:///mnt/cdrom/RPM-GPG-KEY-CentOS-7
      EOF

      yum --disablerepo=\* --enablerepo=cdrom -y reinstall yum
      yum --disablerepo=\* --enablerepo=cdrom -y groupinstall "Minimal Install"

      rm -rf /mnt/cdrom /etc/yum.repos.d/cdrom.repo
    releases:
      - 7

  - trigger: post-packages
    action: |-
      #!/bin/sh
      rm -rf /var/cache/yum
```
