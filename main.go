package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rancher/wharfie/pkg/extract"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/rancher/wharfie/pkg/tarfile"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	version = "v0.0.0"
)

func main() {
	app := cli.NewApp()
	app.Name = "wharfie"
	app.Usage = "pulls and unpacks a container image to the local filesystem"
	app.Description = "Honors K3s/RKE2 style repository rewrites, endpoint overrides, and auth configuration. Supports optional loading from local image tarballs or layer cache."
	app.Version = version
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:     "image",
			Usage:    "Image reference to unpack",
			Required: true,
		},
		cli.StringFlag{
			Name:     "destination",
			Usage:    "Location to place the unpacked image content",
			Required: true,
		},
		cli.StringFlag{
			Name:  "private-registry",
			Usage: "Private registry configuration file",
			Value: "/etc/rancher/common/registries.yaml",
		},
		cli.StringFlag{
			Name:  "images-dir",
			Usage: "Images tarball directory",
		},
		cli.BoolFlag{
			Name:  "cache",
			Usage: "Enable layer cache when image is not available locally",
		},
		cli.StringFlag{
			Name:  "cache-dir",
			Usage: "Layer cache directory",
			Value: "$XDG_CACHE_HOME/rancher/wharfie",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable debug logging",
		},
	}

	if os.Getenv("XDG_CACHE_HOME") == "" {
		os.Setenv("XDG_CACHE_HOME", os.ExpandEnv("$HOME/.cache"))
	}

	if err := app.Run(os.Args); err != nil {
		if !errors.Is(err, context.Canceled) {
			logrus.Fatal(err)
		}
	}
}

func run(clx *cli.Context) error {
	var img v1.Image

	if clx.Bool("debug") {
		logrus.SetLevel(logrus.TraceLevel)
	}

	image, err := name.ParseReference(clx.String("image"))
	if err != nil {
		return err
	}

	destination, err := filepath.Abs(os.ExpandEnv(clx.String("destination")))
	if err != nil {
		return err
	}

	if clx.IsSet("images-dir") {
		imagesDir, err := filepath.Abs(os.ExpandEnv(clx.String("images-dir")))
		if err != nil {
			return err
		}

		i, err := tarfile.FindImage(imagesDir, image)
		if err != nil && !errors.Is(err, tarfile.NotFoundError) {
			return err
		}
		img = i
	}

	if img == nil {
		registry, err := registries.GetPrivateRegistries(clx.String("private-registry"))
		if err != nil {
			return err
		}

		multiKeychain := authn.NewMultiKeychain(registry, authn.DefaultKeychain)

		logrus.Infof("Pulling image %s", image.Name())
		img, err = remote.Image(registry.Rewrite(image), remote.WithAuthFromKeychain(multiKeychain), remote.WithTransport(registry))
		if err != nil {
			return errors.Wrapf(err, "failed to get image %s", image.Name())
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

	return extract.Extract(img, destination)
}
