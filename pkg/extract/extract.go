package extract

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Extract extracts all content from the image to the provided path.
func Extract(img v1.Image, dir string) error {
	dirs := map[string]string{"/": dir}
	return ExtractDirs(img, dirs)
}

// ExtractDirs extracts content from the image, honoring the directory map when
// deciding where on the local filesystem to place the extracted files. For example:
// {"/bin": "/usr/local/bin", "/etc": "/etc", "/etc/rancher": "/opt/rancher/etc"}
func ExtractDirs(img v1.Image, dirs map[string]string) error {
	cleanDirs := make(map[string]string, len(dirs))
	destination := ""

	// Clean the directory map to ensure that source and destination reliably do
	// not have trailing slashes, unless the path is root. This is required to
	// make directory name matching reliable while walking up the source path.
	for s, d := range dirs {
		var err error
		if s != "/" {
			s = strings.TrimRight(s, "/")
		}
		if d != "/" {
			d, err = filepath.Abs(strings.TrimRight(d, "/"))
			if err != nil {
				return errors.Wrap(err, "invalid destination")
			}
		}
		cleanDirs[s] = d
	}

	reader := mutate.Extract(img)
	defer reader.Close()

	// Read from the tar until EOF
	t := tar.NewReader(reader)
	for {
		h, err := t.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		if !strings.HasPrefix(h.Name, "/") {
			h.Name = "/" + h.Name
		}

		// Walk up the source path, finding the longest matching prefix in the map
		for s := filepath.Dir(h.Name); ; s = filepath.Dir(s) {
			if d, ok := cleanDirs[s]; ok {
				destination = filepath.Join(d, strings.TrimLeft(h.Name, s))
				break
			}
		}

		if destination == "" {
			logrus.Debugf("Skipping file %s", h.Name)
			continue
		}

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			logrus.Infof("Extracting file %s to %s", h.Name, destination)

			mode := h.FileInfo().Mode() & 0755
			f, err := os.OpenFile(destination, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
			if err != nil {
				return err
			}

			if _, err = io.Copy(f, t); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}
