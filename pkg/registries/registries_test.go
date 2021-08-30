package registries

import (
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
)

func TestRewrite(t *testing.T) {
	type mss map[string]string

	rewriteTests := []struct {
		registry   string
		rewrites   mss
		imageNames mss
	}{
		{ // syntax error in rewrite. This should log a warning and fail to apply
			registry: "docker.io",
			rewrites: mss{
				"(.*": "docker/$1",
			},
			imageNames: mss{
				"busybox": "index.docker.io/library/busybox:latest",
			},
		},
		{ // no rewrites, everything should come through unmodified
			registry: "docker.io",
			rewrites: mss{},
			imageNames: mss{
				"busybox":             "index.docker.io/library/busybox:latest",
				"registry.local/test": "registry.local/test:latest",
			},
		},
		{ // rewrite docker.io images to prefix "docker/"
			registry: "docker.io",
			rewrites: mss{
				"(.*)": "docker/$1",
			},
			imageNames: mss{
				"busybox":             "index.docker.io/docker/library/busybox:latest",
				"registry.local/test": "registry.local/test:latest",
			},
		},
		{ // ensure that rewrites work with digests
			registry: "docker.io",
			rewrites: mss{
				"(.*)": "docker/$1",
			},
			imageNames: mss{
				"busybox@sha256:82becede498899ec668628e7cb0ad87b6e1c371cb8a1e597d83a47fac21d6af3": "index.docker.io/docker/library/busybox@sha256:82becede498899ec668628e7cb0ad87b6e1c371cb8a1e597d83a47fac21d6af3",
			},
		},
		{ // rewrite registry.local images to prefix "localimages/"
			registry: "registry.local",
			rewrites: mss{
				"(.*)": "localimages/$1",
			},
			imageNames: mss{
				"busybox":             "index.docker.io/library/busybox:latest",
				"registry.local/test": "registry.local/localimages/test:latest",
			},
		},
		{ // rewrite docker.io rancher and longhornio images to unique prefixes; others remain unchanged
			registry: "docker.io",
			rewrites: mss{
				"rancher/(.*)":    "rancher/prod/$1",
				"longhornio/(.*)": "longhornio/staging/$1",
			},
			imageNames: mss{
				"rancher/rancher:v2.5.9":            "index.docker.io/rancher/prod/rancher:v2.5.9",
				"longhornio/longhorn-engine:v1.1.1": "index.docker.io/longhornio/staging/longhorn-engine:v1.1.1",
				"busybox":                           "index.docker.io/library/busybox:latest",
			},
		},
		{ // rewrite docker.io images to prefix "docker.io/"
			registry: "docker.io",
			rewrites: mss{
				"(.*)": "docker.io/$1",
			},
			imageNames: mss{
				"busybox":             "index.docker.io/docker.io/library/busybox:latest",
				"registry.local/test": "registry.local/test:latest",
			},
		},
		{ // rewrite k8s.gcr.io to prefix "k8s.gcr.io/"
			registry: "k8s.gcr.io",
			rewrites: mss{
				"(.*)": "k8s.gcr.io/$1",
			},
			imageNames: mss{
				"busybox":              "index.docker.io/library/busybox:latest",
				"k8s.gcr.io/pause:3.2": "k8s.gcr.io/k8s.gcr.io/pause:3.2",
			},
		},
		{ // rewrite without a trailing slash
			registry: "docker.io",
			rewrites: mss{
				"(.*)": "mirrored-$1",
			},
			imageNames: mss{
				"busybox": "index.docker.io/mirrored-library/busybox:latest",
			},
		},
		{ // rewrite with the match as a prefix instead of suffix.
			// I can't think of why anyone would want to do this though.
			registry: "docker.io",
			rewrites: mss{
				"(.*)": "$1/docker",
			},
			imageNames: mss{
				"busybox": "index.docker.io/library/busybox/docker:latest",
			},
		},
		{ // replace all namespace separators with dashes
			// note that this doesn't work for docker.io, as it has an implicit 'library/' namespace
			// that gets inserted if you don't have a namespace.
			registry: "registry.local",
			rewrites: mss{
				"/": "-",
			},
			imageNames: mss{
				"registry.local/team1/images/test": "registry.local/team1-images-test:latest",
			},
		},
	}

	for _, test := range rewriteTests {
		registry := registry{
			r: &Registry{
				Mirrors: map[string]Mirror{
					test.registry: {
						Endpoints: []string{"https://registry.example.com/v2/"},
						Rewrites:  test.rewrites,
					},
				},
				Configs: map[string]RegistryConfig{},
			},
			t: map[string]*http.Transport{},
			w: map[string]bool{},
		}

		for source, dest := range test.imageNames {
			originalRef, err := name.ParseReference(source)
			if err != nil {
				t.Errorf("failed to parse source reference: %v", err)
				continue
			}
			rewriteRef := registry.Rewrite(originalRef)
			if rewriteRef.Name() != dest {
				t.Errorf("Bad rewrite for %s as %s - got %s, wanted %s", source, originalRef.Name(), rewriteRef.Name(), dest)
			} else {
				t.Logf("OK  rewrite for %s as %s - got %s", source, originalRef.Name(), rewriteRef.Name())
			}
		}
	}
}
