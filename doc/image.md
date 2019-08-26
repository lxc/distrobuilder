# Image

The image section describes the image.

```yaml
image:
    distribution: <string> # required
    architecture: <string>
    description: <string>
    expiry: <string>
    name: <string>
    release: <string>
    serial: <string>
    variant: <string>
```

The fields `distribution`, `architecture`, `description` and `release` are self-explanatory.
If `architecture` is not set, it defaults to the host's architecture.

The `expiry` field describes the image expiry.
The format is `\d+(s|m|h|d|w)` (seconds, minutes, hours, days, weeks), and defaults to 30 days (`30d`).
It's also possible to define multiple such parts, e.g. `1h 30m 10s`.

The `name` field is used in the LXD metadata as well as the output name for LXD unified tarballs.
It defaults to `{{ image.distribution }}-{{ image.release }}-{{ image.architecture_mapped }}-{{ image.variant }}-{{ image.serial }}`.

The `serial` field is the image's serial number.
It can be anything and defaults to `YYYYmmdd_HHMM` (date format).

The `variant` field can be anything and is used in the LXD metadata as well as for [filtering](filter.md).
