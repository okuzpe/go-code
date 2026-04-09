package tools

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRedirectURLBlocksRFC1918(t *testing.T) {
	u, err := url.Parse("http://192.168.0.1/path")
	require.NoError(t, err)
	err = validateRedirectURL(u)
	require.ErrorIs(t, err, errPrivateNetwork)
}

func TestValidateRedirectURLBlocksLoopback(t *testing.T) {
	u, err := url.Parse("http://127.0.0.1:8080/")
	require.NoError(t, err)
	err = validateRedirectURL(u)
	require.ErrorIs(t, err, errPrivateNetwork)
}

func TestValidateRedirectURLBlocksMetadataHost(t *testing.T) {
	u, err := url.Parse("http://169.254.169.254/latest/meta-data/")
	require.NoError(t, err)
	err = validateRedirectURL(u)
	require.Error(t, err)
}

func TestValidateRedirectURLAllowsPublicIP(t *testing.T) {
	u, err := url.Parse("http://8.8.8.8/")
	require.NoError(t, err)
	require.NoError(t, validateRedirectURL(u))
}
