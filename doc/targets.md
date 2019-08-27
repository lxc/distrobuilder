# Targets

The target section is for target dependent files.
Currently, the only supported target is LXC.

```yaml
targets:
    lxc:
        create-message: <string>
        config:
            - type: <string>
              before: <uint>
              after: <uint>
              content: <string>
            - ...
```

The `create-message` field is a string which is displayed after new LXC container has been created.
This string is rendered using pongo2 and can include various fields from the definition file, e.g. `{{ image.description }}`.

`config` is a list of container config options.
The `type` must be `all`, `system` or `user`.

The keys `before` and `after` are used for compatibility.
Currently, the maximum value for compatability is 5.
If your desired compatability level is 3 for example, you would use `before: 4` and `after: 2`.

`content` describes the config which is to be written to the config file.
