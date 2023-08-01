package directorclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/exp/slices"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	boshUAA "github.com/cloudfoundry/bosh-cli/v7/uaa"
	boshHTTPClient "github.com/cloudfoundry/bosh-utils/httpclient"
	boshLog "github.com/cloudfoundry/bosh-utils/logger"
)

type Configuration struct {
	Environment     string `env:"BOSH_ENVIRONMENT"   yaml:"BOSH_ENVIRONMENT"`
	EnvironmentName string `env:"BOSH_ENV_NAME"      yaml:"BOSH_ENV_NAME"`
	AllProxy        string `env:"BOSH_ALL_PROXY"     yaml:"BOSH_ALL_PROXY"`
	Client          string `env:"BOSH_CLIENT"        yaml:"BOSH_CLIENT"`
	ClientSecret    string `env:"BOSH_CLIENT_SECRET" yaml:"BOSH_CLIENT_SECRET"`
	CACertificate   string `env:"BOSH_CA_CERT"       yaml:"BOSH_CA_CERT"`
}

func New(configuration Configuration) (director.Director, error) {
	boshLogger := boshLog.NewLogger(boshLog.LevelError)

	if configuration.AllProxy != "" {
		err := os.Setenv("BOSH_ALL_PROXY", configuration.AllProxy)
		if err != nil {
			return nil, err
		}
		boshHTTPClient.ResetDialerContext()
	}

	directorConfig, err := director.NewConfigFromURL(configuration.Environment)
	if err != nil {
		return nil, fmt.Errorf("failed to get director config from BOSH_ENVIRONMENT: %w", err)
	}
	boshDirectorURL := "https://" + net.JoinHostPort(directorConfig.Host, strconv.Itoa(directorConfig.Port))

	httpClient, err := configuration.httpClient()
	if err != nil {
		return nil, fmt.Errorf("failed to configure HTTP client: %w", err)
	}

	boshCACert, err := os.ReadFile(configuration.CACertificate)
	if err != nil {
		log.Fatal(err)
	}

	uaa, err := configuration.uaaClient(httpClient, boshLogger, boshDirectorURL, boshCACert)
	if err != nil {
		log.Fatal(err)
	}

	directorConfig.TokenFunc = boshUAA.NewClientTokenSession(uaa).TokenFunc
	directorConfig.CACert = string(boshCACert)

	boshFactory := director.NewFactory(boshLogger)
	client, err := boshFactory.New(directorConfig, director.NewNoopTaskReporter(), director.NewNoopFileReporter())
	if err != nil {
		log.Fatal(err)
	}
	return client, nil
}

func (configuration *Configuration) SetFieldsFromEnvironment() error {
	v := reflect.ValueOf(configuration).Elem()

	for i := 0; i < v.NumField(); i++ {
		if !v.Type().Field(i).IsExported() {
			continue
		}
		v.Field(i).SetString(os.Getenv(v.Type().Field(i).Tag.Get("env")))
	}

	return nil
}

func (configuration *Configuration) uaaClient(httpClient *http.Client, boshLogger boshLog.Logger, boshDirectorURL string, boshCACert []byte) (boshUAA.UAA, error) {
	uaaURL, err := configuration.uaaURL(httpClient, boshLogger, boshDirectorURL)
	if err != nil {
		return nil, err
	}

	uaaClientFactory := boshUAA.NewFactory(boshLogger)

	uaaConfig, err := boshUAA.NewConfigFromURL(uaaURL)
	if err != nil {
		return nil, err
	}

	uaaConfig.CACert = string(boshCACert)
	uaaConfig.Client = configuration.Client
	uaaConfig.ClientSecret = configuration.ClientSecret

	return uaaClientFactory.New(uaaConfig)
}

func (configuration *Configuration) uaaURL(httpClient *http.Client, boshLogger boshLog.Logger, boshDirectorURL string) (string, error) {
	unAuthedClient := boshHTTPClient.NewHTTPClient(httpClient, boshLogger)
	unAuthedDirector := director.NewClient(boshDirectorURL, unAuthedClient, director.NewNoopTaskReporter(), director.NewNoopFileReporter(), boshLogger)
	info, err := unAuthedDirector.Info()
	if err != nil {
		log.Fatalf("could not get basic director info: %s", err)
	}
	return getAuthURLFromInfo(info)
}

func (configuration *Configuration) httpClient() (*http.Client, error) {
	if configuration.AllProxy == "" {
		return boshHTTPClient.DefaultClient, nil
	}

	boshAllProxyURL, err := url.Parse(configuration.AllProxy)
	if err != nil {
		log.Fatalf("failed to parse BOSH_ALL_PROXY: %s", err)
	}
	if boshAllProxyURL.User == nil {
		log.Fatal("username not in BOSH_ALL_PROXY")
	}
	privateKeyPath := boshAllProxyURL.Query().Get("private-key")
	if privateKeyPath == "" {
		log.Fatal("required private-key query parameter not in BOSH_ALL_PROXY")
	}
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		log.Fatalf("failed read BOSH_ALL_PROXY private_key: %s", err)
	}
	omPrivateKey, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		log.Fatalf("failed to parse private key from OM_PRIVATE_KEY: %s", err)
	}
	hostKey, err := insecureGetHostKey(omPrivateKey, boshAllProxyURL.User.Username(), boshAllProxyURL.Host)
	if err != nil {
		log.Fatalf("failed to get host key for proxy host OM_PRIVATE_KEY: %s", err)
	}

	socksClient, err := ssh.Dial("tcp", boshAllProxyURL.Host, &ssh.ClientConfig{
		Timeout:         30 * time.Second,
		User:            boshAllProxyURL.User.Username(),
		HostKeyCallback: ssh.FixedHostKey(hostKey),
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(omPrivateKey)},
	})
	if err != nil {
		log.Fatalf("failed to connect to jumpbox: %s", err)
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DialContext: func(_ context.Context, network, addr string) (net.Conn, error) {
				return socksClient.Dial(network, addr)
			},
			TLSHandshakeTimeout: 30 * time.Second,
			DisableKeepAlives:   false,
		},
	}, nil
}

// insecureGetHostKey just returns the key returned by the host and does not
// attempt to ensure the key is from whom it says it is from.
// This is what the BOSH CLI does, so it might be okay.
// Please review this and how the resulting public key is used.
func insecureGetHostKey(signer ssh.Signer, userName, serverURL string) (ssh.PublicKey, error) {
	publicKeyChannel := make(chan ssh.PublicKey, 1)
	defer close(publicKeyChannel)

	dialErrorChannel := make(chan error)
	defer close(dialErrorChannel)

	clientConfig := &ssh.ClientConfig{
		Timeout: time.Minute,
		User:    userName,
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
		defer closeAndIgnoreError(conn)
		dialErrorChannel <- nil
	}()

	return <-publicKeyChannel, <-dialErrorChannel
}

func getAuthURLFromInfo(info director.InfoResp) (string, error) {
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

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}

func overrideEnvironmentWithConfigurationStructure(configStructure any, environ []string) []string {
	v := reflect.ValueOf(configStructure)
	if v.Type().Kind() != reflect.Struct {
		panic("expected configStructure to have reflect.Kind reflect.Struct")
	}
	for fieldIndex := 0; fieldIndex < v.NumField(); fieldIndex++ {
		fieldType := v.Type().Field(fieldIndex)
		if !fieldType.IsExported() {
			continue
		}
		fieldValue := v.Field(fieldIndex)
		envVarNam := fieldType.Tag.Get("env")
		if envVarNam == "" {
			continue
		}
		if fieldValue.Kind() != reflect.String {
			continue
		}
		fieldString := fieldValue.String()
		updatedElement := strings.Join([]string{envVarNam, fieldString}, "=")
		if environIndex := slices.IndexFunc(environ, func(element string) bool {
			return strings.HasPrefix(element, envVarNam+"=")
		}); environIndex >= 0 {
			environ[environIndex] = updatedElement
			continue
		} else if fieldString != "" {
			environ = append(environ, updatedElement)
		}
	}
	return environ
}
