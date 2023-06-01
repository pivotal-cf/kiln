package component

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_wrapFirewallError(t *testing.T) {
	t.Run("dns error", func(t *testing.T) {
		someErr := &url.Error{
			Err: &net.DNSError{
				Err: "some message",
			},
		}
		err := wrapVPNError(someErr)
		_, ok := err.(*vpnError)
		require.True(t, ok)
	})
	t.Run("any other error", func(t *testing.T) {
		someErr := fmt.Errorf("lemon")
		err := wrapVPNError(someErr)
		_, ok := err.(*vpnError)
		require.False(t, ok)
	})
}

func Test_lockFromURLFilename(t *testing.T) {
	const multiPartPath = "bosh-releases/{{.StemcellOS}}/{{.StemcellVersion}}/{{.Name}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz"
	for _, tt := range []struct {
		name string

		pathTemplateString string
		fileName           string

		expError bool
		expLock  Lock
	}{
		{
			name:               "multiple path parts",
			pathTemplateString: multiPartPath,
			fileName:           "mango-1.2.3-sherbet-8.9.tgz",
			expLock: Lock{
				Version:         "1.2.3",
				StemcellVersion: "8.9",
			},
		},

		{
			name:               "v prefix",
			pathTemplateString: "{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz",
			fileName:           "mango-v1.2.3-sherbet-8.9.tgz",
			expLock: Lock{
				Version:         "1.2.3",
				StemcellVersion: "8.9",
			},
		},

		{
			name:               "just release and version",
			pathTemplateString: "{{.Name}}-{{.Version}}.tgz",
			fileName:           "mango-1.2.3.tgz",
			expLock: Lock{
				Version: "1.2.3",
			},
		},

		{
			name:               "v prefix in template",
			pathTemplateString: "{{.Name}}-v{{.Version}}.tgz",
			fileName:           "mango-v1.2.3.tgz",
			expLock: Lock{
				Version: "1.2.3",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lock, err := lockVersionsFromPath(tt.pathTemplateString, tt.fileName)
			if tt.expError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expLock, lock)
		})
	}
}
