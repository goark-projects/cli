# Versioning and Releases

English | [Simplified Chinese](versioning-and-releases.zh-CN.md)

Goark CLI uses Semantic Versioning. CLI releases, project descriptor versions, and lock-file versions are separate contracts and must not be treated as one number.

## Version Contracts

| Contract | Example | Meaning |
| --- | --- | --- |
| CLI release | `v0.0.1` | Version of the `goark` executable and Go module. |
| Build descriptor | `version = 1` | Schema version of `goark.build`. |
| Lock file | `version = 1` | Schema version of `goark.build.lock`. |

Changing the CLI version does not automatically change either file format. A file-format version changes only when its parser contract becomes incompatible.

## Compatibility Before 1.0

- Patch releases such as `v0.0.2` fix defects without intentionally changing supported configuration or command behavior.
- Minor releases such as `v0.1.0` may change unstable APIs or CLI behavior, with migration notes in the changelog.
- Removed behavior is not retained through compatibility shims unless a release note explicitly says otherwise.
- `goark go ...` remains the escape hatch for unmodified official Go behavior.

## Version Resolution

The executable resolves its displayed version in this order:

1. A release-build value injected by GoReleaser.
2. The module version embedded by `go install goark.dev/cli/cmd/goark@<version>`.
3. `devel` for an untagged local build.

The leading `v` is omitted from command output, so tag `v0.0.1` prints `goark 0.0.1`.

## Supported Release Targets

Each release publishes the following archives:

| Operating system | Architectures | Format |
| --- | --- | --- |
| Linux | amd64, arm64 | `.tar.gz` |
| Windows | amd64, arm64 | `.zip` |
| macOS | amd64, arm64 | `.tar.gz` |

`checksums.txt` contains a SHA-256 digest for every archive. V0.0.1 archives are checksum-protected but are not platform-code-signed.

## Release Pipeline

1. Complete feature, test, documentation, and changelog changes on `dev`.
2. Run `GOWORK=off` tests, race detection, vet, workflow validation, and a GoReleaser snapshot.
3. Push `dev` and require the Windows, Ubuntu, macOS, and race jobs to pass.
4. Fast-forward `main` to the verified `dev` commit.
5. Create and push an annotated semantic-version tag from that exact commit.
6. The tag workflow repeats cross-platform validation before publishing archives and checksums.
7. Verify the GitHub Release, downloaded archives, checksums, binary version, and a clean external `go install`.

The release workflow has read-only repository permissions during validation. Only the final publish job receives `contents: write`.

## Installation and Upgrade

Install an exact release for reproducible environments:

```bash
go install goark.dev/cli/cmd/goark@v0.0.1
goark version
```

Use the same command with a newer tag to upgrade. To roll back, reinstall the required earlier tag. Project caches and lock files are not silently rewritten by `go install`.

## Release Records

- [Changelog](../CHANGELOG.md)
- [V0.0.1 release notes](releases/v0.0.1.md)
