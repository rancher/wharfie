package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rancher/wharfie/pkg/credentialprovider/plugin"
	"github.com/rancher/wharfie/pkg/extract"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/rancher/wharfie/pkg/tarfile"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	version = "v0.0.0"
)

func main() {
	app := cli.NewApp()
	app.Name = "wharfie"
	app.Usage = "pulls and unpacks a container image to the local filesystem"
	app.Description = "Supports K3s/RKE2 style repository rewrites, endpoint overrides, and auth configuration. Supports optional loading from local image tarballs or layer cache. Supports Kubelet credential provider plugins."
	app.ArgsUsage = "<image> [<destination>|<source:destination>] [<source:destination>]"
	app.Version = version
	app.Action = run
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "private-registry",
			Usage: "Private registry configuration file",
			Value: "/etc/rancher/common/registries.yaml",
		},
		&cli.StringFlag{
			Name:  "images-dir",
			Usage: "Images tarball directory",
		},
		&cli.BoolFlag{
			Name:  "cache",
			Usage: "Enable layer cache when image is not available locally",
		},
		&cli.StringFlag{
			Name:  "cache-dir",
			Usage: "Layer cache directory",
			Value: "$XDG_CACHE_HOME/rancher/wharfie",
		},
		&cli.StringFlag{
			Name:  "image-credential-provider-config",
			Usage: "Image credential provider configuration file",
		},
		&cli.StringFlag{
			Name:  "image-credential-provider-bin-dir",
			Usage: "Image credential provider binary directory",
		},
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable debug logging",
		},
		&cli.StringFlag{
			Name:  "arch",
			Usage: "Override the machine architecture",
			Value: runtime.GOARCH,
		},
		&cli.StringFlag{
			Name:  "os",
			Usage: "Override the machine operating system",
			Value: runtime.GOOS,
		},
	}

	if os.Getenv("XDG_CACHE_HOME") == "" && os.Getenv("HOME") != "" {
		os.Setenv("XDG_CACHE_HOME", os.ExpandEnv("$HOME/.cache"))
	}

	if err := app.Run(os.Args); err != nil {
		if !errors.Is(err, context.Canceled) {
			logrus.Fatalf("Error: %v", err)
		}
	}
}

func run(clx *cli.Context) error {
	var img v1.Image

	if clx.Args().Len() < 2 {
		fmt.Fprintf(clx.App.Writer, "Incorrect Usage. <image> and <destination> are required arguments.\n\n")
		cli.ShowAppHelpAndExit(clx, 1)
	}

	if clx.Bool("debug") {
		logrus.SetLevel(logrus.TraceLevel)
	}

	ref, err := name.ParseReference(clx.Args().Get(0))
	if err != nil {
		return err
	}

	// destination is one or more bare local paths to extract to on the host, or
	// image-path:local-path pairs if the content should be extracted to specific
	// locations.
	dirs := map[string]string{}
	for i := 1; i < clx.NArg(); i++ {
		var source, destination string
		destination = clx.Args().Get(i)
		parts := strings.SplitN(destination, ":", 2)
		if len(parts) == 2 {
			source, destination = parts[0], parts[1]
		} else {
			source, destination = "/", parts[0]
		}
		destination, err := filepath.Abs(os.ExpandEnv(destination))
		if err != nil {
			return err
		}
		logrus.Infof("Extract mapping %s => %s", source, destination)
		dirs[source] = destination
	}

	if clx.IsSet("images-dir") {
		imagesDir, err := filepath.Abs(os.ExpandEnv(clx.String("images-dir")))
		if err != nil {
			return err
		}

		i, err := tarfile.FindImage(imagesDir, ref)
		if err != nil && !errors.Is(err, tarfile.ErrNotFound) {
			return err
		}
		img = i
	}

	if img == nil {
		registry, err := registries.GetPrivateRegistries(clx.String("private-registry"))
		if err != nil {
			return err
		}

		// Next check Kubelet image credential provider plugins, if configured
		if clx.IsSet("image-credential-provider-config") && clx.IsSet("image-credential-provider-bin-dir") {
			plugins, err := plugin.RegisterCredentialProviderPlugins(clx.String("image-credential-provider-config"), clx.String("image-credential-provider-bin-dir"))
			if err != nil {
				return err
			}
			registry.DefaultKeychain = plugins
		} else {
			// The kubelet image credential provider plugin also falls back to checking legacy Docker credentials, so only
			// explicitly set up the go-containerregistry DefaultKeychain if plugins are not configured.
			// DefaultKeychain tries to read config from the home dir, and will error if HOME isn't set, so also gate on that.
			if os.Getenv("HOME") != "" {
				registry.DefaultKeychain = authn.DefaultKeychain
			}
		}

		logrus.Infof("Pulling image reference %s", ref.Name())
		img, err = registry.Image(ref, remote.WithPlatform(v1.Platform{Architecture: clx.String("arch"), OS: clx.String("os")}))
		if err != nil {
			return errors.Wrapf(err, "failed to get image reference %s", ref.Name())
		}

		if clx.Bool("cache") {
			cacheDir, err := filepath.Abs(os.ExpandEnv(clx.String("cache-dir")))
			if err != nil {
				return err
			}
			logrus.Infof("Using layer cache %s", cacheDir)
			imageCache := cache.NewFilesystemCache(cacheDir)
			img = cache.Image(img, imageCache)
		}
	}

	return extract.ExtractDirs(img, dirs)
}
