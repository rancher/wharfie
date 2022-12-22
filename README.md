# wharfie
Utility libraries to provide additional functionality for users of go-containerregistry. Also includes a basic command-line app demonstrating use of the library code.

### cli

```console
NAME:
   wharfie - pulls and unpacks a container image to the local filesystem

USAGE:
   wharfie [global options] command [command options] <image> <destination>

VERSION:
   v0.3.1

DESCRIPTION:
   Supports K3s/RKE2 style repository rewrites, endpoint overrides, and auth configuration.
   Supports optional loading from local image tarballs or layer cache.
   Supports Kubelet credential provider plugins.

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --private-registry value                   Private registry configuration file (default: "/etc/rancher/common/registries.yaml")
   --images-dir value                         Images tarball directory
   --cache                                    Enable layer cache when image is not available locally
   --cache-dir value                          Layer cache directory (default: "$XDG_CACHE_HOME/rancher/wharfie")
   --image-credential-provider-config value   Image credential provider configuration file
   --image-credential-provider-bin-dir value  Image credential provider binary directory
   --debug                                    Enable debug logging
   --help, -h                                 show help
   --version, -v                              print the version
```

### image credential providers

([KEP-2133](https://github.com/kubernetes/enhancements/issues/2133)) [kubelet image credential providers](https://kubernetes.io/docs/tasks/kubelet-credential-provider/kubelet-credential-provider/) are supported.
At the time of this writing, none of the out-of-tree cloud providers offer standalone binaries. The wharfie docker image (available by running `make package-image`) bundles provider plugins at `/bin/plugins`,
with a sample config file at `/etc/config.yaml`.

More information is available at:
* https://github.com/kubernetes/cloud-provider-aws/tree/master/cmd/ecr-credential-provider
* https://github.com/kubernetes/cloud-provider-gcp/tree/master/cmd/auth-provider-gcp
* https://github.com/kubernetes-sigs/cloud-provider-azure/tree/master/cmd/acr-credential-provider
