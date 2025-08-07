# Install

```sh
go install github.com/modfin/depot/cmd/depot@latest
```

# Run

```sh
depot -r save --lint
```

# Example .depoy.yml

```yaml
dependency:
  ignore:
    - type: go
      name: github.com/goftp/file-driver
      version: "*"
    - type: go
      name: github.com/modfin/epoxy

  licenses:
    - type: go
      name: github.com/goodsign/monday
      version: v1.0.0
      license: [BSD-2-Clause]
```