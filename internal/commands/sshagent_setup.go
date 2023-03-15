package commands

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"

	"github.com/docker/docker/pkg/homedir"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
)

//counterfeiter:generate -o fakes/fake_ssh_client_creator.go --fake-name SshClientCreator . SshClientCreator
type SshClientCreator interface {
	NewClient(rw io.ReadWriter) SshAgent
}

//counterfeiter:generate -o fakes/fake_ssh_agent.go --fake-name SSHAgent . SshAgent
type SshAgent interface {
	Add(key sshagent.AddedKey) error
	List() ([]*sshagent.Key, error)
}

type SSHClientCreator struct{}

func (s SSHClientCreator) NewClient(rw io.ReadWriter) SshAgent {
	return sshagent.NewClient(rw)
}

type Key struct {
	KeyPath   string
	Encrypted bool
}

var StandardSSHKeys = []string{
	"id_rsa",
	"id_dsa",
	"id_ecdsa",
	"id_ed25519",
	"identity",
}

//counterfeiter:generate -o ./fakes/ssh_thing.go --fake-name SshProvider . SshProvider
type SshProvider interface {
	NeedsKeys() (bool, error)
	GetKeys(optionalKeyPaths ...string) (key Key, err error)
	AddKey(key Key, passphrase []byte) error
}

type SshThing struct {
	sshAgent SshAgent
}

func NewSshProvider(creator SshClientCreator) (SshProvider, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return SshThing{}, err
	}
	agentClient := creator.NewClient(conn)
	return SshThing{
		sshAgent: agentClient,
	}, nil
}

func (st SshThing) NeedsKeys() (bool, error) {
	keys, err := st.sshAgent.List()
	if err != nil {
		fmt.Printf("Error listing keys: %s", err)
		return false, err
	}
	return len(keys) == 0, nil
}

func (st SshThing) GetKeys(optionalKeyPaths ...string) (key Key, err error) {
	var keyPaths []string
	if len(optionalKeyPaths) > 0 {
		keyPaths = optionalKeyPaths
	} else {
		homeDir := homedir.Get()

		for _, keyName := range StandardSSHKeys {
			keyPaths = append(keyPaths, path.Join(homeDir, ".ssh", keyName))
		}
	}

	var keyPath string
	for _, curKeyPath := range keyPaths {
		_, err := os.Stat(curKeyPath)
		if err != nil {
			continue
		}
		keyPath = curKeyPath
		break
	}
	if keyPath == "" {
		return Key{}, errors.New("no ssh key found")
	}

	f, err := os.Open(keyPath)
	if err != nil {
		return Key{}, err
	}
	dt, err := io.ReadAll(&io.LimitedReader{R: f, N: 100 * 1024})
	if err != nil {
		return Key{}, err
	}
	_, err = ssh.ParseRawPrivateKey(dt)
	encrypted := false
	if _, ok := err.(*ssh.PassphraseMissingError); ok {
		fmt.Println("passphrase missing error for ", keyPath)
		encrypted = true
	} else if err != nil {
		return Key{}, err
	}
	return Key{KeyPath: keyPath, Encrypted: encrypted}, nil
}

func (st SshThing) AddKey(key Key, passphrase []byte) error {
	f, err := os.Open(key.KeyPath)
	if err != nil {
		return err
	}
	dt, err := io.ReadAll(&io.LimitedReader{R: f, N: 100 * 1024})
	if err != nil {
		fmt.Printf("Error reading key: %s", err)
		return err
	}

	if key.Encrypted {
		decryptedKey, err := ssh.ParseRawPrivateKeyWithPassphrase(dt, passphrase)
		if err != nil {
			return err
		}
		err = st.sshAgent.Add(sshagent.AddedKey{PrivateKey: decryptedKey})
		if err != nil {
			return err
		}
	} else {
		decryptedKey, err := ssh.ParseRawPrivateKey(dt)
		if err != nil {
			return err
		}
		err = st.sshAgent.Add(sshagent.AddedKey{PrivateKey: decryptedKey})
		if err != nil {
			return err
		}
	}
	return nil
}
