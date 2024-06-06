package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/pivotal-cf/kiln/internal/test/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/agent"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

func TestConfiguration_commands(t *testing.T) {
	absoluteTileDirectory := filepath.Join(t.TempDir(), "test")
	require.NoError(t, os.MkdirAll(absoluteTileDirectory, 0o700))

	for _, tt := range []struct {
		Name            string
		Configuration   Configuration
		Result          []string
		ExpErrSubstring string
	}{
		{
			Name: "when the tile path is not absolute",
			Configuration: Configuration{
				AbsoluteTileDirectory: ".",
			},
			ExpErrSubstring: "tile path must be absolute",
		},
		{
			Name: "when no flags are true",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
			},
			Result: nil,
		},
		{
			Name: "when running migrations tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunMigrations:         true,
			},
			Result: []string{"cd /tas/test/migrations", "npm install", "npm test"},
		},
		{
			Name: "when running manifest tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunManifest:           true,
			},
			Result: []string{"cd /tas/test && ginkgo  /tas/test/test/manifest"},
		},
		{
			Name: "when running metadata tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunMetadata:           true,
			},
			Result: []string{"cd /tas/test && ginkgo  /tas/test/test/stability"},
		},
		{
			Name: "when running all tests",
			Configuration: Configuration{
				AbsoluteTileDirectory: absoluteTileDirectory,
				RunAll:                true,
			},
			Result: []string{"cd /tas/test/migrations", "npm install", "npm test", "cd /tas/test && ginkgo  /tas/test/test/stability /tas/test/test/manifest"},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			result, err := tt.Configuration.commands()
			if tt.ExpErrSubstring != "" {
				require.ErrorContains(t, err, tt.ExpErrSubstring)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.Result, result)
			}
		})
	}
}

func Test_configureSession(t *testing.T) {
	t.Run("when ping fails", func(t *testing.T) {
		ctx := context.Background()
		logger := log.New(io.Discard, "", 0)

		client := new(fakes.MobyClient)
		client.PingReturns(types.Ping{}, fmt.Errorf("lemon"))

		fn := func(string) error { panic("don't call this") }

		err := configureSession(ctx, logger, Configuration{}, client, fn)

		require.ErrorContains(t, err, "failed to connect to Docker daemon")
	})
}

func Test_loadImage(t *testing.T) {
	absoluteTileDirectory := filepath.Join(t.TempDir(), "test")
	logger := log.New(io.Discard, "", 0)
	t.Run("when loading a provided test image with a wrong path", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
			ImagePath:             "non-existing",
		}
		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 0,
		})

		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.ErrorContains(t, err, "failed to read image 'non-existing': open non-existing: no such file or directory")

	})
	t.Run(`when loading a provided test image with an existing path`, func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}

		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
			ImagePath:             `assets/alpine.tgz`,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 0,
		})

		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.NoError(t, err)

	})
}

func Test_runTestWithSession(t *testing.T) {
	absoluteTileDirectory := filepath.Join(t.TempDir(), "test")
	logger := log.New(io.Discard, "", 0)

	t.Run("when the command succeeds", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 0,
		})

		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.NoError(t, err)
	})

	t.Run("when the command fails", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 22,
		})

		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.ErrorContains(t, err, "test failed with exit code 22")
	})

	t.Run("when the command fails with an error message", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 22,
			Error: &container.WaitExitError{
				Message: "banana",
			},
		})
		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.ErrorContains(t, err, "test failed with exit code 22: banana")
	})

	t.Run("when fetching container logs fails", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 0,
		})
		client.ContainerLogsReturns(nil, fmt.Errorf("banana"))

		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.ErrorContains(t, err, "container log request failure: ")
		require.ErrorContains(t, err, "banana")
	})

	t.Run("when starting the container container fails", func(t *testing.T) {
		ctx := context.Background()
		out := bytes.Buffer{}
		configuration := Configuration{
			AbsoluteTileDirectory: absoluteTileDirectory,
		}

		client := runTestWithSessionHelper(t, "", container.WaitResponse{
			StatusCode: 0,
		})
		client.ContainerStartReturns(fmt.Errorf("banana"))

		err := runTestWithSession(ctx, logger, &out, client, configuration)("some-session-id")
		require.ErrorContains(t, err, "failed to start test container: ")
		require.ErrorContains(t, err, "banana")
	})
}

func runTestWithSessionHelper(t *testing.T, logs string, response container.WaitResponse) *fakes.MobyClient {
	t.Helper()
	client := new(fakes.MobyClient)
	client.ImageBuildReturns(types.ImageBuildResponse{
		Body: io.NopCloser(strings.NewReader("")),
	}, nil)
	client.ImageLoadReturns(types.ImageLoadResponse{
		Body: io.NopCloser(strings.NewReader("")),
	}, nil)

	client.ContainerStartReturns(nil)
	client.ContainerLogsReturns(io.NopCloser(strings.NewReader(logs)), nil)

	waitResp := make(chan container.WaitResponse)
	waitErr := make(chan error)
	client.ContainerWaitReturns(waitResp, waitErr)

	wg := sync.WaitGroup{}
	wg.Add(1)
	t.Cleanup(func() {
		wg.Wait()
	})
	testCtx, done := context.WithCancel(context.Background())
	go func() {
		defer wg.Done()
		select {
		case waitResp <- response:
		case <-testCtx.Done():
		}
	}()
	t.Cleanup(func() {
		done()
	})
	return client
}

func Test_ensureSSHAgentKeys(t *testing.T) {
	password := func(password []byte, err error) func() ([]byte, error) {
		return func() ([]byte, error) {
			return password, err
		}
	}

	t.Run("when the agent has keys", func(t *testing.T) {
		fakeAgent := new(fakes.SSHAgent)
		fakeAgent.ListReturns(make([]*agent.Key, 2), nil)
		err := ensureSSHAgentKeys(fakeAgent, t.TempDir(), password(nil, nil))
		require.NoError(t, err)
	})
	t.Run("when listing keys fails", func(t *testing.T) {
		fakeAgent := new(fakes.SSHAgent)
		fakeAgent.ListReturns(nil, fmt.Errorf("banana"))
		err := ensureSSHAgentKeys(fakeAgent, t.TempDir(), password(nil, nil))
		require.ErrorContains(t, err, "failed to list keys")
	})
	t.Run("when the agent has no keys", func(t *testing.T) {
		t.Run("when the hidden ssh directory does not exist", func(t *testing.T) {
			fakeAgent := new(fakes.SSHAgent)
			fakeAgent.ListReturns(nil, nil)
			err := ensureSSHAgentKeys(fakeAgent, t.TempDir(), password(nil, nil))

			require.NoError(t, err)
		})
		t.Run("when an unknown id file is found", func(t *testing.T) {
			fakeAgent := new(fakes.SSHAgent)
			fakeAgent.ListReturns(nil, nil)
			home := t.TempDir()
			someFile := filepath.Join(home, ".ssh", "id_peach")
			require.NoError(t, os.MkdirAll(filepath.Dir(someFile), 0o700))
			require.NoError(t, os.WriteFile(someFile, []byte("orange"), 0o600))

			err := ensureSSHAgentKeys(fakeAgent, home, password(nil, nil))

			require.NoError(t, err)
		})
		t.Run("when the key file contents are not a key", func(t *testing.T) {
			fakeAgent := new(fakes.SSHAgent)
			fakeAgent.ListReturns(nil, nil)
			home := t.TempDir()
			someFile := filepath.Join(home, ".ssh", "id_rsa")
			require.NoError(t, os.MkdirAll(filepath.Dir(someFile), 0o700))
			require.NoError(t, os.WriteFile(someFile, []byte("banana"), 0o600))

			err := ensureSSHAgentKeys(fakeAgent, home, password(nil, nil))

			require.ErrorContains(t, err, "failed to read key")
		})
		t.Run("when the key is not encrypted", func(t *testing.T) {
			fakeAgent := new(fakes.SSHAgent)
			fakeAgent.ListReturns(nil, nil)
			home := t.TempDir()
			someFile := filepath.Join(home, ".ssh", "id_rsa")
			require.NoError(t, os.MkdirAll(filepath.Dir(someFile), 0o700))
			require.NoError(t, os.WriteFile(someFile, pemUnencryptedKey.PEMBytes, 0o600))

			err := ensureSSHAgentKeys(fakeAgent, home, password(nil, nil))

			require.NoError(t, err)
		})
		t.Run("when the key is encrypted", func(t *testing.T) {
			t.Run("when the password is correct", func(t *testing.T) {
				fakeAgent := new(fakes.SSHAgent)
				fakeAgent.ListReturns(nil, nil)
				home := t.TempDir()
				someFile := filepath.Join(home, ".ssh", "id_rsa")
				require.NoError(t, os.MkdirAll(filepath.Dir(someFile), 0o700))
				require.NoError(t, os.WriteFile(someFile, pemEncryptedKey.PEMBytes, 0o600))

				err := ensureSSHAgentKeys(fakeAgent, home, password(pemEncryptedKey.EncryptionKey, nil))

				require.NoError(t, err)

				assert.Equal(t, 1, fakeAgent.AddCallCount())
			})
			t.Run("when the password service returns an error", func(t *testing.T) {
				fakeAgent := new(fakes.SSHAgent)
				fakeAgent.ListReturns(nil, nil)

				home := t.TempDir()
				someFile := filepath.Join(home, ".ssh", "id_rsa")
				require.NoError(t, os.MkdirAll(filepath.Dir(someFile), 0o700))
				require.NoError(t, os.WriteFile(someFile, pemEncryptedKey.PEMBytes, 0o600))

				err := ensureSSHAgentKeys(fakeAgent, home, password(nil, fmt.Errorf("lemon")))

				require.ErrorContains(t, err, "failed to get password")
			})
			t.Run("when the password is not correct", func(t *testing.T) {
				fakeAgent := new(fakes.SSHAgent)
				fakeAgent.ListReturns(nil, nil)

				home := t.TempDir()
				someFile := filepath.Join(home, ".ssh", "id_rsa")
				require.NoError(t, os.MkdirAll(filepath.Dir(someFile), 0o700))
				require.NoError(t, os.WriteFile(someFile, pemEncryptedKey.PEMBytes, 0o600))

				err := ensureSSHAgentKeys(fakeAgent, home, password([]byte("banana"), nil))

				require.ErrorContains(t, err, "decryption password incorrect")
			})
		})
	})
}

func Test_addKey(t *testing.T) {
	t.Run("when reading the file fails", func(t *testing.T) {
		fakeAgent := new(fakes.SSHAgent)

		keyFilePath := filepath.Join(t.TempDir(), "does_not_exist")

		err := addKey(fakeAgent, keyFilePath, func() ([]byte, error) { return nil, nil })
		require.Error(t, err)
	})
}

func Test_loadKeyPassword(t *testing.T) {
	isTerm := func(result bool) func(i int) bool {
		return func(i int) bool {
			return result
		}
	}
	readPassword := func(password []byte, err error) func(i int) ([]byte, error) {
		return func(i int) ([]byte, error) {
			return password, err
		}
	}

	t.Run("when the environment variable is set", func(t *testing.T) {
		t.Setenv(sshPasswordEnvVarName, "orange")

		ps := keyPasswordService{
			stdout:       new(bytes.Buffer),
			stdin:        temporaryFile(t, nil),
			isTerm:       isTerm(false),
			readPassword: readPassword(nil, nil),
		}

		result, err := ps.password()

		require.NoError(t, err)
		assert.Equal(t, "orange", string(result))
	})

	t.Run("when the file is a terminal", func(t *testing.T) {
		out := new(bytes.Buffer)
		ps := keyPasswordService{
			stdout:       out,
			stdin:        temporaryFile(t, nil),
			isTerm:       isTerm(true),
			readPassword: readPassword([]byte("peach"), nil),
		}
		result, err := ps.password()

		require.NoError(t, err)
		assert.Equal(t, "peach", string(result))
		assert.Contains(t, out.String(), "Enter password: ")
	})

	t.Run("when the file is not a terminal", func(t *testing.T) {
		out := new(bytes.Buffer)
		ps := keyPasswordService{
			stdout:       out,
			stdin:        temporaryFile(t, []byte("mango\n\n\n")),
			isTerm:       isTerm(false),
			readPassword: readPassword(nil, nil),
		}

		result, err := ps.password()

		require.NoError(t, err)
		assert.Equal(t, "mango", string(result))
	})

	t.Run("when read password fails", func(t *testing.T) {
		out := new(bytes.Buffer)
		ps := keyPasswordService{
			stdout:       out,
			stdin:        temporaryFile(t, nil),
			isTerm:       isTerm(true),
			readPassword: readPassword(nil, errors.New("lemon")),
		}

		_, err := ps.password()

		require.Error(t, err)
	})

	t.Run("when reading stdin never gets a new line", func(t *testing.T) {
		ps := keyPasswordService{
			stdout:       new(bytes.Buffer),
			stdin:        temporaryFile(t, []byte("mango")),
			isTerm:       isTerm(false),
			readPassword: readPassword(nil, nil),
		}

		_, err := ps.password()

		require.ErrorContains(t, err, "failed to read from standard input")
	})
}

func Test_decodeEnvironment(t *testing.T) {
	for _, tt := range []struct {
		Name            string
		In              []string
		Exp             map[string]string
		ExpErrSubstring string
	}{
		{
			Name: "valid variable",
			In:   []string{"fruit=orange"},
			Exp: map[string]string{
				"fruit": "orange",
			},
		},
		{
			Name:            "no separator",
			In:              []string{"fruit:orange"},
			ExpErrSubstring: "environment variables must have the format [key]=[value]",
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := decodeEnvironment(tt.In)
			if tt.ExpErrSubstring != "" {
				require.ErrorContains(t, err, tt.ExpErrSubstring)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.Exp, got)
			}
		})
	}
}

func temporaryFile(t *testing.T, content []byte) *os.File {
	t.Helper()
	dir := t.TempDir()
	stdIn, err := os.Create(filepath.Join(dir, "file.txt"))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = stdIn.Close()
	})
	if len(content) > 0 {
		_, err := stdIn.Write(content)
		if err != nil {
			t.Fatal(err)
		}
		_, err = stdIn.Seek(0, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	return stdIn
}

// from go standard library
// https://github.com/golang/crypto/blob/5d542ad81a58c89581d596f49d0ba5d435481bcf/ssh/testdata/keys.go
var pemEncryptedKey = struct {
	Name              string
	EncryptionKey     []byte
	IncludesPublicKey bool
	PEMBytes          []byte
}{
	Name:          "rsa-encrypted",
	EncryptionKey: []byte("r54-G0pher_t3st$"),
	PEMBytes: []byte(`-----BEGIN RSA PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-128-CBC,3E1714DE130BC5E81327F36564B05462
MqW88sud4fnWk/Jk3fkjh7ydu51ZkHLN5qlQgA4SkAXORPPMj2XvqZOv1v2LOgUV
dUevUn8PZK7a9zbZg4QShUSzwE5k6wdB7XKPyBgI39mJ79GBd2U4W3h6KT6jIdWA
goQpluxkrzr2/X602IaxLEre97FT9mpKC6zxKCLvyFWVIP9n3OSFS47cTTXyFr+l
7PdRhe60nn6jSBgUNk/Q1lAvEQ9fufdPwDYY93F1wyJ6lOr0F1+mzRrMbH67NyKs
rG8J1Fa7cIIre7ueKIAXTIne7OAWqpU9UDgQatDtZTbvA7ciqGsSFgiwwW13N+Rr
hN8MkODKs9cjtONxSKi05s206A3NDU6STtZ3KuPDjFE1gMJODotOuqSM+cxKfyFq
wxpk/CHYCDdMAVBSwxb/vraOHamylL4uCHpJdBHypzf2HABt+lS8Su23uAmL87DR
yvyCS/lmpuNTndef6qHPRkoW2EV3xqD3ovosGf7kgwGJUk2ZpCLVteqmYehKlZDK
r/Jy+J26ooI2jIg9bjvD1PZq+Mv+2dQ1RlDrPG3PB+rEixw6vBaL9x3jatCd4ej7
XG7lb3qO9xFpLsx89tkEcvpGR+broSpUJ6Mu5LBCVmrvqHjvnDhrZVz1brMiQtU9
iMZbgXqDLXHd6ERWygk7OTU03u+l1gs+KGMfmS0h0ZYw6KGVLgMnsoxqd6cFSKNB
8Ohk9ZTZGCiovlXBUepyu8wKat1k8YlHSfIHoRUJRhhcd7DrmojC+bcbMIZBU22T
Pl2ftVRGtcQY23lYd0NNKfebF7ncjuLWQGy+vZW+7cgfI6wPIbfYfP6g7QAutk6W
KQx0AoX5woZ6cNxtpIrymaVjSMRRBkKQrJKmRp3pC/lul5E5P2cueMs1fj4OHTbJ
lAUv88ywr+R+mRgYQlFW/XQ653f6DT4t6+njfO9oBcPrQDASZel3LjXLpjjYG/N5
+BWnVexuJX9ika8HJiFl55oqaKb+WknfNhk5cPY+x7SDV9ywQeMiDZpr0ffeYAEP
LlwwiWRDYpO+uwXHSFF3+JjWwjhs8m8g99iFb7U93yKgBB12dCEPPa2ZeH9wUHMJ
sreYhNuq6f4iWWSXpzN45inQqtTi8jrJhuNLTT543ErW7DtntBO2rWMhff3aiXbn
Uy3qzZM1nPbuCGuBmP9L2dJ3Z5ifDWB4JmOyWY4swTZGt9AVmUxMIKdZpRONx8vz
I9u9nbVPGZBcou50Pa0qTLbkWsSL94MNXrARBxzhHC9Zs6XNEtwN7mOuii7uMkVc
adrxgknBH1J1N+NX/eTKzUwJuPvDtA+Z5ILWNN9wpZT/7ed8zEnKHPNUexyeT5g3
uw9z9jH7ffGxFYlx87oiVPHGOrCXYZYW5uoZE31SCBkbtNuffNRJRKIFeipmpJ3P
7bpAG+kGHMelQH6b+5K1Qgsv4tpuSyKeTKpPFH9Av5nN4P1ZBm9N80tzbNWqjSJm
S7rYdHnuNEVnUGnRmEUMmVuYZnNBEVN/fP2m2SEwXcP3Uh7TiYlcWw10ygaGmOr7
MvMLGkYgQ4Utwnd98mtqa0jr0hK2TcOSFir3AqVvXN3XJj4cVULkrXe4Im1laWgp
-----END RSA PRIVATE KEY-----
`),
}

var pemUnencryptedKey = struct {
	Name     string
	PEMBytes []byte
}{
	Name: "rsa-unencrypted",
	PEMBytes: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQC8A6FGHDiWCSREAXCq6yBfNVr0xCVG2CzvktFNRpue+RXrGs/2
a6ySEJQb3IYquw7HlJgu6fg3WIWhOmHCjfpG0PrL4CRwbqQ2LaPPXhJErWYejcD8
Di00cF3677+G10KMZk9RXbmHtuBFZT98wxg8j+ZsBMqGM1+7yrWUvynswQIDAQAB
AoGAJMCk5vqfSRzyXOTXLGIYCuR4Kj6pdsbNSeuuRGfYBeR1F2c/XdFAg7D/8s5R
38p/Ih52/Ty5S8BfJtwtvgVY9ecf/JlU/rl/QzhG8/8KC0NG7KsyXklbQ7gJT8UT
Ojmw5QpMk+rKv17ipDVkQQmPaj+gJXYNAHqImke5mm/K/h0CQQDciPmviQ+DOhOq
2ZBqUfH8oXHgFmp7/6pXw80DpMIxgV3CwkxxIVx6a8lVH9bT/AFySJ6vXq4zTuV9
6QmZcZzDAkEA2j/UXJPIs1fQ8z/6sONOkU/BjtoePFIWJlRxdN35cZjXnBraX5UR
fFHkePv4YwqmXNqrBOvSu+w2WdSDci+IKwJAcsPRc/jWmsrJW1q3Ha0hSf/WG/Bu
X7MPuXaKpP/DkzGoUmb8ks7yqj6XWnYkPNLjCc8izU5vRwIiyWBRf4mxMwJBAILa
NDvRS0rjwt6lJGv7zPZoqDc65VfrK2aNyHx2PgFyzwrEOtuF57bu7pnvEIxpLTeM
z26i6XVMeYXAWZMTloMCQBbpGgEERQpeUknLBqUHhg/wXF6+lFA+vEGnkY+Dwab2
KCXFGd+SQ5GdUcEMe9isUH6DYj/6/yCDoFrXXmpQb+M=
-----END RSA PRIVATE KEY-----
`),
}
