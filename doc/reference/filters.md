# Filters

Filters can be used to restrict certain sections from being run or being applied.
There are three filters, `releases`, `architectures`, and `variants`, and each filter takes a list.

Here's an example:

```yaml
releases:
- v1
- v2
architectures:
- x86_64
variants:
- cloud
```

In the above case, the section will only be applied or run if the release is v1 or v2, the architecture is x86_64 _and_ the variant is cloud.

Filters can be applied to each item individually in the lists of following sections:

- files
- sets (packages)
- actions
- repositories
