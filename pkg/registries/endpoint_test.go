package registries

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/dynamiclistener/factory"
	"github.com/sirupsen/logrus"
)

const localhost = "127-0-0-1.sslip.io"

func TestImage(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	imageTests := map[string]struct {
		images   []string
		rewrites map[string]string
	}{
		"Pull busybox without rewrite on default endpoint": {
			images: []string{
				"library/busybox:latest",
			},
			rewrites: map[string]string{
				"^library/(.*)": "bogus-image-prefix/$1",
			},
		},
	}

	for testName, test := range imageTests {
		t.Run(testName, func(t *testing.T) {
			rs, as, mux := newServers(t, "127.0.0.1:443", true, true, true)
			defer rs.Close()
			defer as.Close()

			regHost, regEndpoint := getHostEndpoint(rs.Listener.Addr().String(), true, false)
			authHost, authEndpoint := getHostEndpoint(as.Listener.Addr().String(), true, false)

			t.Logf("INFO: %s registry %s at %s, auth %s at %s, scheme %q", t.Name(), regHost, regEndpoint, authHost, authEndpoint, "Basic")

			mux.Handle("/v2/", serveRegistry(t, "Basic", authEndpoint+"/auth"))
			mux.Handle("/auth/", serveAuth(t))

			r := &registry{
				DefaultKeychain: authn.DefaultKeychain,
				Registry: &Registry{
					Mirrors: map[string]Mirror{
						regHost: Mirror{
							Endpoints: []string{regHost + ":443"},
							Rewrites:  test.rewrites,
						},
					},
					Configs: map[string]RegistryConfig{
						regHost: RegistryConfig{
							Auth: &AuthConfig{Username: "user", Password: "pass"},
							TLS:  &TLSConfig{InsecureSkipVerify: true},
						},
						regHost + ":443": RegistryConfig{
							Auth: &AuthConfig{Username: "user", Password: "pass"},
							TLS:  &TLSConfig{InsecureSkipVerify: true},
						},
					},
				},
				transports: map[string]*http.Transport{},
			}

			for _, refStr := range test.images {
				t.Run(refStr, func(t *testing.T) {
					ref, err := name.ParseReference(regHost + "/" + refStr)
					if err != nil {
						t.Fatalf("FATAL: Failed to parse reference: %v", err)
					}

					// Target the only supported dummy platform, regardless of what we're running on
					image, err := r.Image(ref, remote.WithPlatform(v1.Platform{Architecture: "amd64", OS: "linux"}))
					if err != nil {
						t.Fatalf("FATAL: Failed to get image: %v", err)
					}

					// confirm that we can get the manifest
					_, err = image.Manifest()
					if err != nil {
						t.Fatalf("FATAL: Failed to get manifest: %v", err)
					}

					// confirm that we can get the config file
					_, err = image.ConfigFile()
					if err != nil {
						t.Fatalf("FATAL: Failed to get config file: %v", err)
					}

					t.Logf("OK: %s", ref)
				})
			}
		})
	}
}

func TestEndpoint(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	endpointTests := map[string]struct {
		address      string // the address to bind to
		explicitPort bool   // whether or not to include the port in the URL, even if it is default for the scheme
		registryTLS  bool   // enable TLS for registry endpoint
		authTLS      bool   // enable TLS for auth endpoint
		sameAddress  bool   // use the same endpoint for both registry and auth
		authScheme   string // scheme to use for authentication (none/basic/bearer)
	}{
		"http anonymous":          {"127.0.0.1:80", false, false, false, true, ""},
		"http basic+local":        {"127.0.0.1:80", false, false, false, true, "Basic"},
		"http basic+http":         {"127.0.0.1:80", false, false, false, false, "Basic"},
		"http basic+https":        {"127.0.0.1:80", false, false, true, false, "Basic"},
		"http bearer+local":       {"127.0.0.1:80", false, false, false, true, "Bearer"},
		"http bearer+http":        {"127.0.0.1:80", false, false, false, false, "Bearer"},
		"http bearer+https":       {"127.0.0.1:80", false, false, true, false, "Bearer"},
		"http:80 anonymous":       {"127.0.0.1:80", true, false, false, true, ""},
		"http:80 basic+local":     {"127.0.0.1:80", true, false, false, true, "Basic"},
		"http:80 basic+http":      {"127.0.0.1:80", true, false, false, false, "Basic"},
		"http:80 basic+https":     {"127.0.0.1:80", true, false, true, false, "Basic"},
		"http:80 bearer+local":    {"127.0.0.1:80", true, false, false, true, "Bearer"},
		"http:80 bearer+http":     {"127.0.0.1:80", true, false, false, false, "Bearer"},
		"http:80 bearer+https":    {"127.0.0.1:80", true, false, true, false, "Bearer"},
		"http:rand anonymous":     {"127.0.0.1:0", true, false, false, true, ""},
		"http:rand basic+local":   {"127.0.0.1:0", true, false, false, true, "Basic"},
		"http:rand basic+http":    {"127.0.0.1:0", true, false, false, false, "Basic"},
		"http:rand basic+https":   {"127.0.0.1:0", true, false, true, false, "Basic"},
		"http:rand bearer+local":  {"127.0.0.1:0", true, false, false, true, "Bearer"},
		"http:rand bearer+http":   {"127.0.0.1:0", true, false, false, false, "Bearer"},
		"http:rand bearer+https":  {"127.0.0.1:0", true, false, true, false, "Bearer"},
		"https anonymous":         {"127.0.0.1:443", false, true, true, true, ""},
		"https basic+local":       {"127.0.0.1:443", false, true, true, true, "Basic"},
		"https basic+http":        {"127.0.0.1:443", false, true, false, false, "Basic"},
		"https basic+https":       {"127.0.0.1:443", false, true, true, false, "Basic"},
		"https bearer+local":      {"127.0.0.1:443", false, true, true, true, "Bearer"},
		"https bearer+http":       {"127.0.0.1:443", false, true, false, false, "Bearer"},
		"https bearer+https":      {"127.0.0.1:443", false, true, true, false, "Bearer"},
		"https:443 anonymous":     {"127.0.0.1:443", true, true, true, true, ""},
		"https:443 basic+local":   {"127.0.0.1:443", true, true, true, true, "Basic"},
		"https:443 basic+http":    {"127.0.0.1:443", true, true, false, false, "Basic"},
		"https:443 basic+https":   {"127.0.0.1:443", true, true, true, false, "Basic"},
		"https:443 bearer+local":  {"127.0.0.1:443", true, true, true, true, "Bearer"},
		"https:443 bearer+http":   {"127.0.0.1:443", true, true, false, false, "Bearer"},
		"https:443 bearer+https":  {"127.0.0.1:443", true, true, true, false, "Bearer"},
		"https:rand anonymous":    {"127.0.0.1:0", true, true, true, true, ""},
		"https:rand basic+local":  {"127.0.0.1:0", true, true, true, true, "Basic"},
		"https:rand basic+http":   {"127.0.0.1:0", true, true, false, false, "Basic"},
		"https:rand basic+https":  {"127.0.0.1:0", true, true, true, false, "Basic"},
		"https:rand bearer+local": {"127.0.0.1:0", true, true, true, true, "Bearer"},
		"https:rand bearer+http":  {"127.0.0.1:0", true, true, false, false, "Bearer"},
		"https:rand bearer+https": {"127.0.0.1:0", true, true, true, false, "Bearer"},
	}

	for testName, test := range endpointTests {
		t.Run(testName, func(t *testing.T) {
			if test.sameAddress && test.registryTLS != test.authTLS {
				t.Fatal("FATAL: Invalid test case: sameAddress is true, but registryTLS != authTLS")
			}

			rs, as, mux := newServers(t, test.address, test.registryTLS, test.authTLS, test.sameAddress)
			defer rs.Close()
			defer as.Close()

			regHost, regEndpoint := getHostEndpoint(rs.Listener.Addr().String(), test.registryTLS, test.explicitPort)
			authHost, authEndpoint := getHostEndpoint(as.Listener.Addr().String(), test.authTLS, test.explicitPort)

			t.Logf("INFO: %s registry %s at %s, auth %s at %s, scheme %q", t.Name(), regHost, regEndpoint, authHost, authEndpoint, test.authScheme)

			mux.Handle("/v2/", serveRegistry(t, test.authScheme, authEndpoint+"/auth"))
			mux.Handle("/auth/", serveAuth(t))

			r := &registry{
				DefaultKeychain: authn.DefaultKeychain,
				Registry: &Registry{
					Mirrors: map[string]Mirror{
						defaultRegistry: Mirror{
							Endpoints: []string{regEndpoint},
						},
						regHost: Mirror{
							Endpoints: []string{regEndpoint},
						},
					},
					Configs: map[string]RegistryConfig{
						regHost: RegistryConfig{
							Auth: &AuthConfig{Username: "user", Password: "pass"},
							TLS:  &TLSConfig{InsecureSkipVerify: true},
						},
					},
				},
				transports: map[string]*http.Transport{},
			}

			// disable TLS verification for the auth endpoint too, if it's separate
			if !test.sameAddress {
				r.Registry.Configs[authHost] = RegistryConfig{TLS: &TLSConfig{InsecureSkipVerify: true}}
			}

			imageRefs := []string{
				defaultRegistry + "/library/busybox:latest",
				regHost + "/library/busybox:latest",
			}

			// When using the default port for a scheme, confirm that the image can be pulled from the bare hostname,
			// even if the port was explicitly included in the registry config.
			if _, port, _ := net.SplitHostPort(test.address); port != "0" && test.explicitPort {
				imageRefs = append(imageRefs, localhost+"/library/busybox:latest")
				r.Registry.Configs[localhost] = RegistryConfig{TLS: &TLSConfig{InsecureSkipVerify: true}}
			}

			for _, refStr := range imageRefs {
				t.Run(refStr, func(t *testing.T) {
					ref, err := name.ParseReference(refStr)
					if err != nil {
						t.Fatalf("FATAL: Failed to parse reference: %v", err)
					}

					// Target the only supported dummy platform, regardless of what we're running on
					image, err := r.Image(ref, remote.WithPlatform(v1.Platform{Architecture: "amd64", OS: "linux"}))
					if err != nil {
						t.Fatalf("FATAL: Failed to get image: %v", err)
					}

					// confirm that we can get the manifest
					_, err = image.Manifest()
					if err != nil {
						t.Fatalf("FATAL: Failed to get manifest: %v", err)
					}

					// confirm that we can get the config file
					_, err = image.ConfigFile()
					if err != nil {
						t.Fatalf("FATAL: Failed to get config file: %v", err)
					}

					t.Logf("OK: %s", ref)
				})
			}
		})
	}
}

// newServers creates and starts httptest.Server instances for the registry and auth endpoints,
// returning both the servers, and a http.ServeMux used by both servers.  If sameAddress is true,
// the authentication server is local to the registry - the same server instance is returned for
// both registry and auth, and the authTLS settings are ignored.  If sameAddress is false, a second
// server is started on a random port.
func newServers(t *testing.T, registryAddress string, registryTLS bool, authTLS bool, sameAddress bool) (*httptest.Server, *httptest.Server, *http.ServeMux) {
	l, err := net.Listen("tcp", registryAddress)
	if err != nil {
		t.Fatalf("FATAL: Failed to listen on %s: %v", registryAddress, err)
	}

	// Create a unique dummy CA and cert for this test's servers, if necessary
	var tlsConfig *tls.Config
	if registryTLS || authTLS {
		caCert, caKey, err := factory.GenCA()
		if err != nil {
			t.Fatalf("FATAL: Failed to generate CA: %v", err)
		}

		cfg := cert.Config{
			CommonName:   localhost,
			Organization: []string{t.Name()},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			AltNames: cert.AltNames{
				DNSNames: []string{localhost},
				IPs:      []net.IP{net.IPv4(127, 0, 0, 1)},
			},
		}
		serverCert, err := cert.NewSignedCert(cfg, caKey, caCert, caKey)
		if err != nil {
			t.Fatalf("FATAL: Failed to generate certificate: %v", err)
		}

		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{
				{Certificate: [][]byte{serverCert.Raw}, Leaf: serverCert, PrivateKey: caKey},
				{Certificate: [][]byte{caCert.Raw}, Leaf: caCert},
			},
		}
	}

	mux := http.NewServeMux()
	rs := &httptest.Server{
		Listener:    l,
		Config:      &http.Server{Handler: mux},
		EnableHTTP2: true,
		TLS:         tlsConfig,
	}
	if registryTLS {
		rs.StartTLS()
	} else {
		rs.Start()
	}
	if sameAddress {
		return rs, rs, mux
	}

	as := httptest.NewUnstartedServer(mux)
	as.EnableHTTP2 = true
	as.TLS = tlsConfig
	if authTLS {
		as.StartTLS()
	} else {
		as.Start()
	}
	return rs, as, mux
}

// getHostEndpoint returns both the bare request host value, and the endpoint URL, for the given address.
// If tls is true, the scheme is https, otherwise http.
// If explicitPort is true, the port will be included in the host value, even if it would not
// normally be included due to being the default port for the scheme. The port is always included
// if it is not the default port for the scheme.
func getHostEndpoint(addr string, tls, explicitPort bool) (string, string) {
	_, port, _ := net.SplitHostPort(addr)
	host := localhost
	scheme := "http"

	if tls {
		scheme = "https"
		if port != "443" || explicitPort {
			host = host + ":" + port
		}
	} else {
		if port != "80" || explicitPort {
			host = host + ":" + port
		}
	}

	return host, scheme + "://" + host
}

// serveRegistry serves requests to the registry endpoint
// If authScheme is set and the request does not have an authorization header, the request will
// be responded to with a requst for authentication.
// Otherwise, a few canned registry API responses will be served; just enough to satisfy the tests.
func serveRegistry(t *testing.T, authScheme, realm string) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Add("Docker-Distribution-Api-Version", "registry/2")

		if authScheme != "" && req.Header.Get("Authorization") == "" {
			var scope string
			if req.URL.Path == "/v2/" {
				scope = "registry:catalog"
			} else if strings.HasPrefix(req.URL.Path, "/v2/library/busybox") {
				scope = "repository:library/busybox"
				switch req.Method {
				case http.MethodGet, http.MethodHead:
					scope += ":pull"
				case http.MethodPost, http.MethodPut, http.MethodPatch:
					scope += ":push,pull"
				case http.MethodDelete:
					scope += ":delete"
				}
			} else {
				resp.WriteHeader(http.StatusForbidden)
				return
			}
			resp.Header().Add("WWW-Authenticate", fmt.Sprintf(`%s realm="%s",service="registry",scope="%s"`, authScheme, realm, scope))
			resp.Header().Add("Content-Type", "application/json")
			resp.WriteHeader(http.StatusUnauthorized)
			resp.Write([]byte(`{"errors":[{"code":"UNAUTHORIZED","message":"authentication required","detail":null}]}`))
			return
		}

		switch req.URL.Path {
		case "/v2/":
			resp.Header().Add("Content-Type", "application/json")
			resp.Write([]byte(`{}`))
		case "/v2/library/busybox/manifests/latest":
			resp.Header().Add("Content-Type", "application/vnd.docker.distribution.manifest.list.v2+json")
			resp.Write([]byte(manifestList))
		case "/v2/library/busybox/manifests/sha256:5cd3db04b8be5773388576a83177aff4f40a03457a63855f4b9cbe30542b9a43":
			resp.Header().Add("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			resp.Write([]byte(manifest))
		case "/v2/library/busybox/blobs/sha256:8135583d97feb82398909c9c97607159e6db2c4ca2c885c0b8f590ee0f9fe90d":
			resp.Header().Add("Content-Type", "application/octet-stream")
			resp.Write([]byte(config))
		default:
			resp.WriteHeader(http.StatusNotFound)
		}
	})
}

// serveAuth serves requests to the authorization service endpoint.
// It does not actually validate any credentials; any request with an Authorization header will be granted a dummy token.
func serveAuth(t *testing.T) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			resp.WriteHeader(http.StatusMethodNotAllowed)
		} else if auth := req.Header.Get("Authorization"); auth != "" {
			if b64, ok := strings.CutPrefix(auth, "Basic "); ok {
				if b, err := base64.StdEncoding.DecodeString(b64); err == nil {
					auth = string(b)
				}
			}
			params := req.URL.Query()
			t.Logf("Got auth request: %+v for %+v", params, auth)
			if service := params["service"]; len(service) != 1 || service[0] != "registry" {
				resp.WriteHeader(http.StatusNotFound)
			} else if scope := params["scope"]; len(scope) != 1 || !(scope[0] == "registry:catalog" || strings.HasPrefix(scope[0], "repository:library/busybox:")) {
				resp.WriteHeader(http.StatusNotFound)
			} else {
				resp.Header().Add("Content-Type", "application/json")
				resp.Write([]byte(fmt.Sprintf(`{"token": "abc", "access_token": "123", "expires_in": 300, "issued_at": "%s"}`, time.Now().Format(time.RFC3339))))
			}
		} else {
			resp.WriteHeader(http.StatusForbidden)
		}
	})
}

// a canned single-arch manifest list for the busybox image's latest tag
var manifestList = `{
  "manifests": [
    {
      "digest": "sha256:5cd3db04b8be5773388576a83177aff4f40a03457a63855f4b9cbe30542b9a43",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      },
      "size": 528
    }
  ],
  "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
  "schemaVersion": 2
}`

// a canned manifest for the busybox image's latest tag
var manifest = `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
   "config": {
      "mediaType": "application/vnd.docker.container.image.v1+json",
      "size": 1457,
      "digest": "sha256:8135583d97feb82398909c9c97607159e6db2c4ca2c885c0b8f590ee0f9fe90d"
   },
   "layers": [
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 2591575,
         "digest": "sha256:325d69979d33f72bfd1d30d420b8ec7f130919916fd02238ba23e4a22d753ed8"
      }
   ]
}`

// a canned config blob for the busybox image's latest tag
var config = `{"architecture":"amd64","config":{"Hostname":"","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["sh"],"Image":"sha256:505de91dcca928e5436702f887bbd8b81be91e719b552fb5c64e34234d22ac86","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":null},"container":"ffeefc40361ae173c8c4a1c2bad0f899f4de97601938eab16b5d019bdf2fa5f3","container_config":{"Hostname":"ffeefc40361a","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/bin/sh","-c","#(nop) ","CMD [\"sh\"]"],"Image":"sha256:505de91dcca928e5436702f887bbd8b81be91e719b552fb5c64e34234d22ac86","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":{}},"created":"2023-05-19T20:19:22.751398522Z","docker_version":"20.10.23","history":[{"created":"2023-05-19T20:19:22.642507645Z","created_by":"/bin/sh -c #(nop) ADD file:cfd4bc7e9470d1298c9d4143538a77aa9aedd74f96aa5a3262cf8714c6fc3ec6 in / "},{"created":"2023-05-19T20:19:22.751398522Z","created_by":"/bin/sh -c #(nop)  CMD [\"sh\"]","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:9547b4c33213e630a0ca602a989ecc094e042146ae8afa502e1e65af6473db03"]}}`
