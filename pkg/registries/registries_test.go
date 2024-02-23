package registries

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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
				assert.NoError(t, err, "Failed to parse source reference %s", source)

				rewriteRef := registry.rewrite(originalRef)
				assert.Equal(t, dest, rewriteRef.Name(), "Bad rewrite for %s as %s", source, originalRef.Name())
				t.Logf("OK rewrite for %s as %s - got %s", source, originalRef.Name(), rewriteRef.Name())
			}
		})
	}
}

func TestEndpoints(t *testing.T) {
	type msr map[string]RegistryConfig
	type msm map[string]Mirror

	endpointTests := map[string]struct {
		imageName  string
		configs    msr
		mirrors    msm
		endpoints  []endpoint
		tlsconfigs []*tls.Config
	}{
		"no config, default endpoint": {
			imageName: "busybox",
			endpoints: []endpoint{
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"local registry with only the default endpoint": {
			imageName: "registry.example.com/busybox",
			endpoints: []endpoint{
				{url: mustParseURL("https://registry.example.com/v2")},
			},
		},
		"local registry with TLS verification disabled": {
			imageName: "registry.example.com/busybox",
			configs:   msr{"registry.example.com": RegistryConfig{TLS: &TLSConfig{InsecureSkipVerify: true}}},
			endpoints: []endpoint{
				{url: mustParseURL("https://registry.example.com/v2")},
			},
			tlsconfigs: []*tls.Config{
				{InsecureSkipVerify: true},
			},
		},
		"local registry with TLS verification disabled in wildcard": {
			imageName: "registry.example.com/busybox",
			configs:   msr{"*": RegistryConfig{TLS: &TLSConfig{InsecureSkipVerify: true}}},
			endpoints: []endpoint{
				{url: mustParseURL("https://registry.example.com/v2")},
			},
			tlsconfigs: []*tls.Config{
				{InsecureSkipVerify: true},
			},
		},
		"local registry with TLS verification disabled in wildcard but not for this registry": {
			imageName: "registry.example.com/busybox",
			configs: msr{
				"*":                    RegistryConfig{TLS: &TLSConfig{InsecureSkipVerify: true}},
				"registry.example.com": RegistryConfig{}},
			endpoints: []endpoint{
				{url: mustParseURL("https://registry.example.com/v2")},
			},
			tlsconfigs: []*tls.Config{
				{InsecureSkipVerify: false},
			},
		},
		"local registry with custom endpoint": {
			imageName: "registry.example.com/busybox",
			mirrors:   msm{"registry.example.com": Mirror{Endpoints: []string{"http://registry.example.com:5000/v2"}}},
			endpoints: []endpoint{
				{url: mustParseURL("http://registry.example.com:5000/v2")},
				{url: mustParseURL("https://registry.example.com/v2")},
			},
		},
		"local registry with custom endpoint with trailing slash": {
			imageName: "registry.example.com/busybox",
			mirrors:   msm{"registry.example.com": Mirror{Endpoints: []string{"http://registry.example.com:5000/v2/"}}},
			endpoints: []endpoint{
				{url: mustParseURL("http://registry.example.com:5000/v2")},
				{url: mustParseURL("https://registry.example.com/v2")},
			},
		},
		"config, but not for the registry we're pulling from": {
			imageName: "busybox",
			mirrors:   msm{"registry.example.com": Mirror{Endpoints: []string{"https://registry.example.com/v2"}}},
			endpoints: []endpoint{
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"config for docker.io, plus default endpoint": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"https://docker.example.com/v2"}}},
			endpoints: []endpoint{
				{url: mustParseURL("https://docker.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"multiple endpoints for docker.io, plus default endpoint": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"https://docker1.example.com/v2", "https://docker2.example.com/v2"}}},
			endpoints: []endpoint{
				{url: mustParseURL("https://docker1.example.com/v2")},
				{url: mustParseURL("https://docker2.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"wildcard registry plus default": {
			imageName: "busybox",
			mirrors:   msm{"*": Mirror{Endpoints: []string{"https://registry.example.com/v2"}}},
			endpoints: []endpoint{
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
			endpoints: []endpoint{
				{url: mustParseURL("https://docker.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"confirm that bad URLs are skipped": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"https://docker1.example.com/v2", "https://user:bad{@docker2.example.com"}}},
			endpoints: []endpoint{
				{url: mustParseURL("https://docker1.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"confirm that relative URLs are skipped": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"https://docker1.example.com/v2", "docker2.example.com/v2", "/v2"}}},
			endpoints: []endpoint{
				{url: mustParseURL("https://docker1.example.com/v2")},
				{url: mustParseURL("https://docker2.example.com/v2")},
				{url: mustParseURL("https://index.docker.io/v2")},
			},
		},
		"confirm that endpoints missing scheme are not skipped": {
			imageName: "registry.example.com/busybox",
			mirrors:   msm{"registry.example.com": Mirror{Endpoints: []string{"registry.example.com:5000/v2"}}},
			endpoints: []endpoint{
				{url: mustParseURL("https://registry.example.com:5000/v2")},
				{url: mustParseURL("https://registry.example.com/v2")},
			},
		},
		"confirm that localhost with odd ports uses http": {
			imageName: "localhost:5000/busybox",
			endpoints: []endpoint{
				{url: mustParseURL("http://localhost:5000/v2")},
			},
		},
		"confirm that localhost with https port uses https": {
			imageName: "localhost:443/busybox",
			endpoints: []endpoint{
				{url: mustParseURL("https://localhost:443/v2")},
			},
		},
		"confirm that loopback with odd ports uses http": {
			imageName: "127.0.0.1:5000/busybox",
			endpoints: []endpoint{
				{url: mustParseURL("http://127.0.0.1:5000/v2")},
			},
		},
		"confirm that loopback with https port uses https": {
			imageName: "127.0.0.1:443/busybox",
			endpoints: []endpoint{
				{url: mustParseURL("https://127.0.0.1:443/v2")},
			},
		},
		"confirm that creds are used for the default endpoint": {
			imageName: "busybox",
			configs:   msr{"docker.io": RegistryConfig{Auth: &AuthConfig{Username: "user", Password: "pass"}}},
			endpoints: []endpoint{
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
			endpoints: []endpoint{
				{
					url:  mustParseURL("https://docker1.example.com/v2"),
					auth: &authn.Basic{Username: "user", Password: "pass"},
				},
				{
					url: mustParseURL("https://index.docker.io/v2"),
				},
			},
		},
		"confirm that creds are used from wildcard config": {
			imageName: "busybox",
			mirrors:   msm{"*": Mirror{Endpoints: []string{"https://registry.example.com/v2"}}},
			configs:   msr{"*": RegistryConfig{Auth: &AuthConfig{Username: "user", Password: "pass"}}},
			endpoints: []endpoint{
				{
					url:  mustParseURL("https://registry.example.com/v2"),
					auth: &authn.Basic{Username: "user", Password: "pass"},
				},
				{
					url:  mustParseURL("https://index.docker.io/v2"),
					auth: &authn.Basic{Username: "user", Password: "pass"},
				},
			},
		},
		"confirm that non-default schemes and ports are honored for mirrors and configs": {
			imageName: "busybox",
			mirrors:   msm{"docker.io": Mirror{Endpoints: []string{"http://docker1.example.com:5000/v2"}}},
			configs:   msr{"docker1.example.com:5000": RegistryConfig{Auth: &AuthConfig{Username: "user", Password: "pass"}}},
			endpoints: []endpoint{
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
			assert.NoError(t, err, "Failed to parse test reference for %v", test.imageName)

			endpoints, err := registry.getEndpoints(ref)
			assert.NoError(t, err, "Failed to get endpoints for %s", ref)

			// Compare endpoint URLs
			var expectedURLs, endpointURLs []string
			for _, e := range test.endpoints {
				expectedURLs = append(expectedURLs, e.url.String())
			}
			for _, e := range endpoints {
				endpointURLs = append(endpointURLs, e.url.String())
			}
			assert.Equal(t, expectedURLs, endpointURLs, "Unexpected endpoints for %s", ref)

			for i, endpoint := range endpoints {
				// Compare endpoint auths
				if i < len(test.endpoints) {
					expectedAuth, err := getAuthConfig(test.endpoints[i], ref)
					assert.NoError(t, err, "Failed to get auth for expected endpoint %d for %s", i, ref)

					epAuth, err := getAuthConfig(endpoint, ref)
					assert.NoError(t, err, "Failed to get auth for test endpoint %d for %s", i, ref)
					assert.Equal(t, expectedAuth, epAuth, "Unexpected auth for endpoint %d for %s", i, ref)
				}

				// Compare endpoint TLS
				if i < len(test.tlsconfigs) {
					tlsConfig, err := registry.getTLSConfig(endpoint.url)
					assert.NoError(t, err, "Failed to get tlsconfig for test endpoint %d for %s", i, ref)
					assert.Equal(t, test.tlsconfigs[i], tlsConfig, "Unexpected tlsconfig for endpoint %d for %s", i, ref)
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
