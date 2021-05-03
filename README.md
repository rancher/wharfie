# wharfie
Utility libraries to provide additional functionality for users of go-containerregistry. Also includes a basic command-line app demonstrating use of the library code.

### cli

```console
NAME:
   wharfie - pulls and unpacks a container image to the local filesystem

USAGE:
   wharfie [global options] command [command options] [arguments...]

VERSION:
   v0.0.1-dirty

DESCRIPTION:
   Honors K3s/RKE2 style repository rewrites, endpoint overrides, and auth configuration. Supports optional loading from local image tarballs or layer cache.

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --image value             Image reference to unpack
   --destination value       Location to place the unpacked image content
   --private-registry value  Private registry configuration file (default: "/etc/rancher/common/registries.yaml")
   --images-dir value        Images tarball directory
   --cache                   Enable layer cache when image is not available locally
   --cache-dir value         Layer cache directory (default: "$XDG_CACHE_HOME/rancher/wharfie")
   --debug                   Enable debug logging
   --help, -h                show help
   --version, -v             print the version
```
