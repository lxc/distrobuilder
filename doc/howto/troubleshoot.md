# Troubleshoot `distrobuilder`

This section covers some of the most commonly encountered problems and gives instructions for resolving them.

## Cannot install into target

> Error `Cannot install into target '/var/cache/distrobuilder.123456789/rootfs' mounted with noexec or nodev`

You have installed `distrobuilder` into a LXD container and you are trying to run it. `distrobuilder` does not run in a LXD container. Run `distrobuilder` on the host, or in a VM.

## Classic confinement

> Error `error: This revision of snap "distrobuilder" was published using classic confinement`

You are trying to install the `distrobuilder` snap package. The `distrobuilder` snap package has been configured to use the `classic` confinement. Therefore, when you install it, you have to add the flag `--classic` as shown above in the instructions.

## Must be root

> Error `You must be root to run this tool`

You must be _root_ in order to run the `distrobuilder` tool. The tool runs commands such as `mknod` that require administrative privileges. Use `sudo` when running `distrobuilder`.

## Kernel settings

> Error `SQUASHFS error: xz decompression failed, data probably corrupt`

If running distrobuilder using WSL2 (Windows Subsystem for Linux) with an unmodified kernel an error occurs during extracting `rootfs.img` out of Rocky Linux image.<br>Solution: compile new WSL2 kernel mit setting `CONFIG_XZ_DEC_X86=y`
