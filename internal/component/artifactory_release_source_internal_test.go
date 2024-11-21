package component

import (
	"fmt"
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
