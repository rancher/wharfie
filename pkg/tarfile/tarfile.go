package tarfile

import (
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4"
	"github.com/pkg/errors"
	"github.com/rancher/wharfie/pkg/util"
	"github.com/sirupsen/logrus"
)

const (
	// The zstd decoder will attempt to use up to 1GB memory for streaming operations by default,
	// which is excessive and will OOM low-memory devices.
	// NOTE: This must be at least as large as the window size used when compressing tarballs, or you
	// will see a "window size exceeded" error when decompressing. The zstd CLI tool uses 4MB by
	// default; the --long option defaults to 27 or 128M, which is still too much for a Pi3. 32MB
	// (--long=25) has been tested to work acceptably while still compressing by an additional 3-6% on
	// our datasets.
	MaxDecoderMemory = 1 << 25
	ExtensionList    = ".tar .tar.lz4 .tar.bz2 .tbz .tar.gz .tgz .tar.zst .tzst" // keep this in sync with the decompressor list
)

var (
	NotFoundError = errors.New("image not found")
)

func FindImage(imagesDir string, imageRef name.Reference) (v1.Image, error) {
	imageTag, ok := imageRef.(name.Tag)
	if !ok {
		return nil, fmt.Errorf("no local image available for %s: reference is not a tag", imageRef.Name())
	}

	if _, err := os.Stat(imagesDir); err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrapf(NotFoundError, "no local image available for %s: directory %s does not exist", imageTag.Name(), imagesDir)
		}
		return nil, err
	}

	logrus.Infof("Checking local image archives in %s for %s", imagesDir, imageTag.Name())

	// Walk the images dir to get a list of tar files
	files := map[string]os.FileInfo{}
	if err := filepath.Walk(imagesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files[path] = info
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Try to find the requested tag in each file, moving on to the next if there's an error
	for fileName := range files {
		img, err := findImage(fileName, imageTag)
		if img != nil {
			logrus.Debugf("Found %s in %s", imageTag.Name(), fileName)
			return img, nil
		}
		if err != nil {
			logrus.Infof("Failed to find %s in %s: %v", imageTag.Name(), fileName, err)
		}
	}
	return nil, errors.Wrapf(NotFoundError, "no local image available for %s: not found in any file in %s", imageTag.Name(), imagesDir)
}

func findImage(fileName string, imageTag name.Tag) (v1.Image, error) {
	var opener tarball.Opener
	switch {
	case util.HasSuffixI(fileName, ".txt"):
		return nil, nil
	case util.HasSuffixI(fileName, ".tar"):
		opener = func() (io.ReadCloser, error) {
			return os.Open(fileName)
		}
	case util.HasSuffixI(fileName, ".tar.lz4"):
		opener = func() (io.ReadCloser, error) {
			file, err := os.Open(fileName)
			if err != nil {
				return nil, err
			}
			zr := lz4.NewReader(file)
			return SplitReadCloser(zr, file), nil
		}
	case util.HasSuffixI(fileName, ".tar.bz2", ".tbz"):
		opener = func() (io.ReadCloser, error) {
			file, err := os.Open(fileName)
			if err != nil {
				return nil, err
			}
			zr := bzip2.NewReader(file)
			return SplitReadCloser(zr, file), nil
		}
	case util.HasSuffixI(fileName, ".tar.gz", ".tgz"):
		opener = func() (io.ReadCloser, error) {
			file, err := os.Open(fileName)
			if err != nil {
				return nil, err
			}
			zr, err := gzip.NewReader(file)
			if err != nil {
				return nil, err
			}
			return MultiReadCloser(zr, file), nil
		}
	case util.HasSuffixI(fileName, "tar.zst", ".tzst"):
		opener = func() (io.ReadCloser, error) {
			file, err := os.Open(fileName)
			if err != nil {
				return nil, err
			}
			zr, err := zstd.NewReader(file, zstd.WithDecoderMaxMemory(MaxDecoderMemory))
			if err != nil {
				return nil, err
			}
			return ZstdReadCloser(zr, file), nil
		}
	default:
		return nil, fmt.Errorf("unhandled file type; supported extensions: " + ExtensionList)
	}

	img, err := tarball.Image(opener, &imageTag)
	if err != nil {
		return nil, err
	}
	return img, nil
}
