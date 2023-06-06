package registries

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sirupsen/logrus"
)

func TestRewrite(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	type mss map[string]string

	rewriteTests := map[string]struct {
		registry   string
		rewrites   mss
		imageNames mss
	}{
		"syntax error in rewrite, log a warning and fail to apply": {
			registry: "docker.io",
			rewrites: mss{
				"(.*": "docker/$1",
			},
			imageNames: mss{
				"busybox": "index.docker.io/library/busybox:latest",
			},
		},
		"no rewrites, unmodified": {
			registry: "docker.io",
			rewrites: mss{},
			imageNames: mss{
				"busybox":             "index.docker.io/library/busybox:latest",
				"registry.local/test": "registry.local/test:latest",
			},
		},
		"rewrite docker.io images to prefix \"docker/\"": {
			registry: "docker.io",
			rewrites: mss{
				"(.*)": "docker/$1",
			},
			imageNames: mss{
				"busybox":             "index.docker.io/docker/library/busybox:latest",
				"registry.local/test": "registry.local/test:latest",
			},
		},
		"ensure that rewrites work with digests": {
			registry: "docker.io",
			rewrites: mss{
				"(.*)": "docker/$1",
			},
			imageNames: mss{
				"busybox@sha256:82becede498899ec668628e7cb0ad87b6e1c371cb8a1e597d83a47fac21d6af3": "index.docker.io/docker/library/busybox@sha256:82becede498899ec668628e7cb0ad87b6e1c371cb8a1e597d83a47fac21d6af3",
			},
		},
		"rewrite registry.local images to prefix \"localimages/\"": {
			registry: "registry.local",
			rewrites: mss{
				"(.*)": "localimages/$1",
			},
			imageNames: mss{
				"busybox":             "index.docker.io/library/busybox:latest",
				"registry.local/test": "registry.local/localimages/test:latest",
			},
		},
		"rewrite docker.io rancher and longhornio images to unique prefixes; others remain unchanged": {
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
		"rewrite docker.io images to prefix \"docker.io/\"": {
			registry: "docker.io",
			rewrites: mss{
				"(.*)": "docker.io/$1",
			},
			imageNames: mss{
				"busybox":             "index.docker.io/docker.io/library/busybox:latest",
				"registry.local/test": "registry.local/test:latest",
			},
		},
		"rewrite registry.k8s.io to prefix \"registry.k8s.io/\"": {
			registry: "registry.k8s.io",
			rewrites: mss{
				"(.*)": "registry.k8s.io/$1",
			},
			imageNames: mss{
				"busybox":                   "index.docker.io/library/busybox:latest",
				"registry.k8s.io/pause:3.2": "registry.k8s.io/registry.k8s.io/pause:3.2",
			},
		},
		"rewrite without a trailing slash": {
			registry: "docker.io",
			rewrites: mss{
				"(.*)": "mirrored-$1",
			},
			imageNames: mss{
				"busybox": "index.docker.io/mirrored-library/busybox:latest",
			},
		},
		"rewrite with the match as a prefix instead of suffix": {
			// I can't think of why anyone would want to do this though.
			registry: "docker.io",
			rewrites: mss{
				"(.*)": "$1/docker",
			},
			imageNames: mss{
				"busybox": "index.docker.io/library/busybox/docker:latest",
			},
		},
		"replace all namespace separators with dashes": {
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

	for testName, test := range rewriteTests {
		t.Run(testName, func(t *testing.T) {
			registry := registry{
				Registry: &Registry{
					Mirrors: map[string]Mirror{
						test.registry: {
							Endpoints: []string{"https://registry.example.com/v2/"},
							Rewrites:  test.rewrites,
						},
					},
					Configs: map[string]RegistryConfig{},
				},
				transports: map[string]*http.Transport{},
			}

			for source, dest := range test.imageNames {
				originalRef, err := name.ParseReference(source)
				if err != nil {
					t.Fatalf("FATAL: Failed to parse source reference: %v", err)
				}
				rewriteRef := registry.rewrite(originalRef)
				if rewriteRef.Name() != dest {
					t.Fatalf("FATAL: Bad rewrite for %s as %s - got %s, wanted %s", source, originalRef.Name(), rewriteRef.Name(), dest)
				} else {
					t.Logf("INFO:   OK rewrite for %s as %s - got %s", source, originalRef.Name(), rewriteRef.Name())
				}
			}
		})
	}
}

func TestEndpoints(t *testing.T) {
	type msr map[string]RegistryConfig
	type msm map[string]Mirror

	endpointTests := map[string]struct {
		imageName string
		configs   msr
		mirrors   msm
		expected  []endpoint
	}{
		"no config, default endpoint": {
			imageName: "busybox",
			expected: []endpoint{
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"local registry with only the default endpoint": {
			imageName: "registry.example.com/busybox",
			expected: []endpoint{
				{url: mustParseURL("https://registry.example.com/v2")},
			},
		},
		"local registry with custom endpoint": {
			imageName: "registry.example.com/busybox",
			mirrors:   msm{"registry.example.com": Mirror{Endpoints: []string{"http://registry.example.com:5000/v2"}}},
			expected: []endpoint{
				{url: mustParseURL("http://registry.example.com:5000/v2")},
				{url: mustParseURL("https://registry.example.com/v2")},
			},
		},
		"local registry with custom endpoint with trailing slash": {
			imageName: "registry.example.com/busybox",
			mirrors:   msm{"registry.example.com": Mirror{Endpoints: []string{"http://registry.example.com:5000/v2/"}}},
			expected: []endpoint{
				{url: mustParseURL("http://registry.example.com:5000/v2")},
				{url: mustParseURL("https://registry.example.com/v2")},
			},
		},
		"config, but not for the registry we're pulling from": {
			imageName: "busybox",
			mirrors:   msm{"registry.example.com": Mirror{Endpoints: []string{"https://registry.example.com/v2"}}},
			expected: []endpoint{
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"config for docker.io, plus default endpoint": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"https://docker.example.com/v2"}}},
			expected: []endpoint{
				{url: mustParseURL("https://docker.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"multiple endpoints for docker.io, plus default endpoint": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"https://docker1.example.com/v2", "https://docker2.example.com/v2"}}},
			expected: []endpoint{
				{url: mustParseURL("https://docker1.example.com/v2")},
				{url: mustParseURL("https://docker2.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"wildcard registry plus default": {
			imageName: "busybox",
			mirrors:   msm{"*": Mirror{Endpoints: []string{"https://registry.example.com/v2"}}},
			expected: []endpoint{
				{url: mustParseURL("https://registry.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"wildcard endpoint plus docker.io; only docker.io should be used": {
			imageName: "busybox",
			mirrors: msm{
				"*":         Mirror{Endpoints: []string{"https://registry.example.com/v2"}},
				"docker.io": Mirror{Endpoints: []string{"https://docker.example.com/v2"}},
			},
			expected: []endpoint{
				{url: mustParseURL("https://docker.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"confirm that bad URLs are skipped": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"https://docker1.example.com/v2", "https://user:bad{@docker2.example.com"}}},
			expected: []endpoint{
				{url: mustParseURL("https://docker1.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"confirm that relative URLs are skipped": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"https://docker1.example.com/v2", "docker2.example.com/v2", "/v2"}}},
			expected: []endpoint{
				{url: mustParseURL("https://docker1.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"confirm that endpoints missing scheme are skipped": {
			imageName: "registry.example.com/busybox",
			mirrors:   msm{"registry.example.com": Mirror{Endpoints: []string{"registry.example.com:5000/v2"}}},
			expected: []endpoint{
				{url: mustParseURL("https://registry.example.com/v2")},
			},
		},
		"confirm that creds are used for the default endpoint": {
			imageName: "busybox",
			configs:   msr{"docker.io": RegistryConfig{Auth: &AuthConfig{Username: "user", Password: "pass"}}},
			expected: []endpoint{
				{
					url:  mustParseURL("https://index.docker.io/v2"),
					auth: &authn.Basic{Username: "user", Password: "pass"},
				},
			},
		},
		"confirm that creds are used for custom endpoints": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"https://docker1.example.com/v2"}}},
			configs:   msr{"docker1.example.com": RegistryConfig{Auth: &AuthConfig{Username: "user", Password: "pass"}}},
			expected: []endpoint{
				{
					url:  mustParseURL("https://docker1.example.com/v2"),
					auth: &authn.Basic{Username: "user", Password: "pass"},
				},
				{
					url: mustParseURL("https://index.docker.io/v2"),
				},
			},
		},
		"confirm that non-default schemes and ports are honored for mirrors and configs": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"http://docker1.example.com:5000/v2"}}},
			configs:   msr{"docker1.example.com:5000": RegistryConfig{Auth: &AuthConfig{Username: "user", Password: "pass"}}},
			expected: []endpoint{
				{
					url:  mustParseURL("http://docker1.example.com:5000/v2"),
					auth: &authn.Basic{Username: "user", Password: "pass"},
				},
				{
					url: mustParseURL("https://index.docker.io/v2"),
				},
			},
		},
	}

	for testName, test := range endpointTests {
		t.Run(testName, func(t *testing.T) {
			registry := registry{
				Registry: &Registry{
					Mirrors: test.mirrors,
					Configs: test.configs,
				},
				transports: map[string]*http.Transport{},
			}

			ref, err := name.ParseReference(test.imageName)
			if err != nil {
				t.Fatalf("FATAL: Failed to parse test refrence: %v", err)
			}

			endpoints, err := registry.getEndpoints(ref)
			if err != nil {
				t.Fatalf("FATAL: Failed to get endpoints for %s: %v", ref, err)
			}
			if expected, got := len(test.expected), len(endpoints); expected != got {
				t.Fatalf("FATAL: Expected %d endpoints, got %d", expected, got)
			}
			for i, endpoint := range endpoints {
				if test.expected[i].url.String() != endpoint.url.String() {
					t.Fatalf("FATAL: Expected endpoint[%d] url %v, got %v", i, test.expected[i].url, endpoint.url)
				}

				expectedAuth, err := getAuthConfig(test.expected[i], ref)
				if err != nil {
					t.Fatalf("FATAL: Failed to get auth for expected endpoint: %v", err)
				}

				epAuth, err := getAuthConfig(endpoint, ref)
				if err != nil {
					t.Fatalf("FATAL: Failed to get auth for test endpoint: %v", err)
				}

				if !reflect.DeepEqual(expectedAuth, epAuth) {
					t.Fatalf("FATAL: Expected endpoint[%d] auth %#v, got %#v", i, expectedAuth, epAuth)
				}
			}
		})
	}
}

func getAuthConfig(resolver authn.Keychain, ref name.Reference) (*authn.AuthConfig, error) {
	auth, err := resolver.Resolve(ref.Context())
	if err != nil {
		return nil, err
	}
	return auth.Authorization()
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		logrus.Fatalf("FATAL: Failed to parse url %s: %v", s, err)
	}
	return u
}
