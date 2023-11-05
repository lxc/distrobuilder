# Actions

```yaml
actions:
    - trigger: <string> # required
      action: |-
        #!/bin/bash
        echo "Run me"
      architectures: <array> # filter
      releases: <array> # filter
      variants: <array> # filter
```

Actions are scripts that are to be run after certain steps during the building process.
Each action has two fields, `trigger` and `action`, as well as some filters.
The `trigger` field describes the step after which the `action` is to be run.
Valid triggers are:

* `post-unpack`
* `post-update`
* `post-packages`
* `post-files`

The above list also shows the order in which the actions are processed.

After the root file system has been unpacked, all `post-unpack` actions are run.

After the package manager has updated all packages, (given that `packages.update` is `true`), all `post-update` actions are run.
After the package manager has installed the requested packages, all `post-packages` actions are run.
For more on `packages`, see [packages](packages.md).

And last, after the `files` section has been processed, all `post-files` actions are run.
This action runs only for `build-lxc`, `build-incus`, `pack-lxc`, and `pack-lxd`.
For more on `files`, see [generators](generators.md).
