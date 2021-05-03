package extract

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/sirupsen/logrus"
)

func Extract(img v1.Image, dir string) error {
	dirs := map[string]string{"/": dir}
	return ExtractDirs(img, dirs)
}

func ExtractDirs(img v1.Image, dirs map[string]string) error {
	reader := mutate.Extract(img)
	defer reader.Close()

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

		target := ""
		for n := filepath.Dir(h.Name); ; n = filepath.Dir(n) {
			if t, ok := dirs[n]; ok {
				if !strings.HasSuffix(t, "/") {
					t = t + "/"
				}
				target = strings.Replace(h.Name, n, t, 1)
				break
			}
		}

		logrus.Infof("%s => %s", h.Name, target)

		if target == "" {
			continue
		}

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			logrus.Infof("Extracting file %s to %s", h.Name, target)

			mode := h.FileInfo().Mode() & 0755
			f, err := os.OpenFile(target, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
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
