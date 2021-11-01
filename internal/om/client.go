package om

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	boshdir "github.com/cloudfoundry/bosh-cli/director"
	boshuaa "github.com/cloudfoundry/bosh-cli/uaa"
	boshhttpclient "github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/pivotal-cf/om/api"
	"github.com/pivotal-cf/om/network"
	"golang.org/x/crypto/ssh"
)

type ClientConfiguration struct {
	Target     string `long:"om-target"      env:"OM_TARGET"`
	Username   string `long:"om-username"    env:"OM_USERNAME"`
	Password   string `long:"om-password"    env:"OM_PASSWORD"`
	CACert     string `long:"om-ca-cert"     env:"OM_CA_CERT"`
	PrivateKey string `long:"om-private-key" env:"OM_PRIVATE_KEY"`

	SkipSSLValidation bool `long:"om-skip-ssl-validation" env:"OM_SKIP_SSL_VALIDATION" default:"false"`
}

func (conf ClientConfiguration) API() (api.Api, error) {
	var (
		// OM_CONNECT_TIMEOUT
		connectTimeout = 10 * time.Second

		// OM_REQUEST_TIMEOUT
		requestTimeout = 1800 * time.Second

		unauthenticatedClient, authedClient, unauthenticatedProgressClient, authedProgressClient interface {
			Do(*http.Request) (*http.Response, error)
		}
	)

	var err error
	unauthenticatedClient, err = network.NewUnauthenticatedClient(conf.Target, conf.SkipSSLValidation, conf.CACert, connectTimeout, requestTimeout)
	if err != nil {
		return api.Api{}, err
	}

	authedClient, err = network.NewOAuthClient(conf.Target, conf.Username, conf.Password, "", "", conf.SkipSSLValidation, conf.CACert, connectTimeout, requestTimeout)
	if err != nil {
		return api.Api{}, err
	}

	unauthenticatedProgressClient = network.NewProgressClient(unauthenticatedClient, os.Stderr)
	authedProgressClient = network.NewProgressClient(authedClient, os.Stderr)

	return api.New(api.ApiInput{
		Client:                 authedClient,
		UnauthedClient:         unauthenticatedClient,
		ProgressClient:         authedProgressClient,
		UnauthedProgressClient: unauthenticatedProgressClient,
		Logger:                 log.New(os.Stderr, "", 0),
	}), nil
}

type GetBoshEnvironmentAndSecurityRootCACertificateProvider interface {
	GetSecurityRootCACertificate() (string, error)
	GetBoshEnvironment() (api.GetBoshEnvironmentOutput, error)
}

func BoshDirector(omConfig ClientConfiguration, omAPI GetBoshEnvironmentAndSecurityRootCACertificateProvider) (boshdir.Director, error) {
	omPrivateKey, err := ssh.ParsePrivateKey([]byte(omConfig.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key from OM_PRIVATE_KEY: %s", err)
	}

	target, err := url.Parse(omConfig.Target)
	if err != nil {
		return nil, fmt.Errorf("failed to parse om target: %s", err)
	}
	serverURL := target.Hostname() + ":22"

	hostKey, err := insecureGetHostKey(omPrivateKey, serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get om ssh host key: %s", err)
	}

	socksClient, err := ssh.Dial("tcp", serverURL, &ssh.ClientConfig{
		Timeout:         30 * time.Second,
		User:            "ubuntu",
		HostKeyCallback: ssh.FixedHostKey(hostKey),
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(omPrivateKey)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ops manager: %s", err)
	}

	boshRootCA, err := omAPI.GetSecurityRootCACertificate()
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(boshRootCA)) {
		return nil, fmt.Errorf("failed to append ops manager root CA")
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				ClientCAs:          pool,
				InsecureSkipVerify: true,
			},
			DialContext: func(_ context.Context, network, addr string) (net.Conn, error) {
				return socksClient.Dial(network, addr)
			},
			TLSHandshakeTimeout: 30 * time.Second,
			DisableKeepAlives:   false,
		},
	}

	boshEnv, err := omAPI.GetBoshEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to get bosh env: %s", err)
	}

	directorConfig, err := boshdir.NewConfigFromURL(boshEnv.Environment)
	if err != nil {
		return nil, err
	}

	boshLogger := boshlog.NewLogger(boshlog.LevelError)
	boshFactory := boshdir.NewFactory(boshLogger)

	boshDirectorURL := fmt.Sprintf("https://%s:%d", directorConfig.Host, directorConfig.Port)

	unAuthedDirector := boshdir.NewClient(boshDirectorURL, boshhttpclient.NewHTTPClient(httpClient, boshLogger),
		boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter(), boshLogger,
	)

	info, err := unAuthedDirector.Info()
	if err != nil {
		return nil, fmt.Errorf("could not get basic director info: %s", err)
	}

	uaaClientFactory := boshuaa.NewFactory(boshLogger)

	uaaURL, err := getAuthURLFromInfo(info)
	if err != nil {
		return nil, fmt.Errorf("could not get basic director info: %s", err)
	}

	uaaConfig, err := boshuaa.NewConfigFromURL(uaaURL)
	if err != nil {
		return nil, err
	}

	uaaConfig.CACert = boshRootCA
	uaaConfig.Client = boshEnv.Client
	uaaConfig.ClientSecret = boshEnv.ClientSecret

	uaa, err := uaaClientFactory.New(uaaConfig)
	if err != nil {
		return nil, fmt.Errorf("could not build uaa auth from director info: %s", err)
	}

	directorConfig.TokenFunc = boshuaa.NewClientTokenSession(uaa).TokenFunc
	directorConfig.CACert = uaaConfig.CACert

	dir, err := boshFactory.New(directorConfig, boshdir.NewNoopTaskReporter(), boshdir.NewNoopFileReporter())
	if err != nil {
		return nil, err
	}

	return dir, nil
}

// insecureGetHostKey just returns the key returned by the host and does not
// attempt to ensure the key is from who it says it is from. This is what the
// bosh CLI does, so it seems to be secure enough.
func insecureGetHostKey(signer ssh.Signer, serverURL string) (ssh.PublicKey, error) {
	publicKeyChannel := make(chan ssh.PublicKey, 1)
	defer close(publicKeyChannel)

	dialErrorChannel := make(chan error)
	defer close(dialErrorChannel)

	clientConfig := &ssh.ClientConfig{
		Timeout: time.Minute,
		User:    "ubuntu",
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			publicKeyChannel <- key
			return nil
		},
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}

	go func() {
		conn, err := ssh.Dial("tcp", serverURL, clientConfig)
		if err != nil {
			publicKeyChannel <- nil
			dialErrorChannel <- err
			return
		}
		defer func() {
			_ = conn.Close()
		}()
		dialErrorChannel <- nil
	}()

	return <-publicKeyChannel, <-dialErrorChannel
}

func getAuthURLFromInfo(info boshdir.InfoResp) (string, error) {
	v, ok := info.Auth.Options["url"]
	if !ok {
		return "", errors.New("missing uaa auth url")
	}
	s, ok := v.(string)
	if !ok {
		return "", errors.New("uaa auth url has the wrong type")
	}
	return s, nil
}
