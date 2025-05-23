package plugin

import (
	"flag"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/google/go-containerregistry/pkg/authn"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	kubecredentialprovider "k8s.io/kubernetes/pkg/credentialprovider"
	kubeplugin "k8s.io/kubernetes/pkg/credentialprovider/plugin"
)

type pluginWrapper struct {
	k kubecredentialprovider.DockerKeyring
}

// Explicit interface checks
var _ authn.Keychain = &pluginWrapper{}

// RegisterCredentialProviderPlugins loads the provided configuration into the credentialprovider plugin registry
// If the configuration is not valid or any configured plugins are missing, an error will be raised.
func RegisterCredentialProviderPlugins(imageCredentialProviderConfigFile, imageCredentialProviderBinDir string) (*pluginWrapper, error) {
	klogSetup()
	// Upstream code does not check if the functions are ever nil before calling them, so stubs are required.
	blankTokenFunc := func(_, _ string, _ *authenticationv1.TokenRequest) (*authenticationv1.TokenRequest, error) {
		return nil, fmt.Errorf("get service account token is not implemented")
	}
	blankSAFunc := func(_, _ string) (*corev1.ServiceAccount, error) {
		return nil, fmt.Errorf("get service account is not implemented")
	}
	if err := kubeplugin.RegisterCredentialProviderPlugins(imageCredentialProviderConfigFile, imageCredentialProviderBinDir, blankTokenFunc, blankSAFunc); err != nil {
		return nil, errors.Wrap(err, "failed to register CRI auth plugins")
	}
	return &pluginWrapper{k: kubecredentialprovider.NewDefaultDockerKeyring()}, nil
}

// Resolve returns an authenticator for the authn.Keychain interface. The authenticator provides
// credentials to a registry by calling the credentialprovider plugin registry's Lookup method,
// which in turn consults the configuration and executes plugins to obtain credentials.
func (p *pluginWrapper) Resolve(target authn.Resource) (authn.Authenticator, error) {
	// Lookup may provide multiple AuthConfigs (for credential rotation support) but the Keychain interface only allows us to return one.
	if configs, ok := p.k.Lookup(target.String()); ok {
		return authn.FromConfig(authn.AuthConfig{
			Username:      configs[0].Username,
			Password:      configs[0].Password,
			Auth:          configs[0].Auth,
			IdentityToken: configs[0].IdentityToken,
			RegistryToken: configs[0].RegistryToken,
		}), nil
	}

	return authn.Anonymous, nil
}

// klogSetup syncs the klog verbosity to the current Logrus log level. This is necessary because the
// auth plugin stuff all uses klog/v2 and there's no good translation layer between logrus and klog.
func klogSetup() {
	klogFlags := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(klogFlags)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		_ = klogFlags.Set("v", "9")
	}
	_ = klogFlags.Parse(nil)
}
