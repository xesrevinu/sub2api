package tlsfingerprint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokBuildProfileMatchesCapturedRustls(t *testing.T) {
	profile := GrokBuildProfile()
	require.Equal(t, GrokBuildProfileName, profile.Name)
	require.Equal(t, []string{"h2", "http/1.1"}, profile.ALPNProtocols)
	require.False(t, profile.EnableGREASE)
	require.Equal(t, []uint16{0x1302, 0x1301, 0x1303, 0xc02c, 0xc02b, 0xcca9, 0xc030, 0xc02f, 0xcca8, 0x00ff}, profile.CipherSuites)
	require.Equal(t, []uint16{0x001d, 0x0017, 0x0018}, profile.Curves)
	require.Equal(t, []uint16{0x0503, 0x0403, 0x0807, 0x0806, 0x0805, 0x0804, 0x0601, 0x0501, 0x0401}, profile.SignatureAlgorithms)
	require.Equal(t, []uint16{0, 10, 51, 45, 35, 43, 11, 16, 23, 5, 13}, profile.Extensions)
	require.True(t, WantsHTTP2(profile))
	require.False(t, WantsHTTP2(&Profile{Name: "http1", ALPNProtocols: []string{"http/1.1"}}))
	require.False(t, WantsHTTP2(nil))

	spec := buildClientHelloSpecFromProfile(profile)
	require.Equal(t, profile.CipherSuites, spec.CipherSuites)
	require.NotEmpty(t, spec.Extensions)
}
