package tlsfingerprint

// GrokBuildProfileName identifies the rustls 0.23 ClientHello captured from
// local Grok Build 1.0.3 (reqwest 0.12, rustls-tls + aws-lc-rs, HTTP/2).
const GrokBuildProfileName = "Grok Build (rustls 0.23)"

// GrokBuildProfile returns the TLS fingerprint used by official Grok Build.
// Cipher suites, groups, signature algorithms, and extension order were
// captured from ~/.grok/bin/grok 1.0.3 on macOS aarch64. SNI (type 0) is
// prepended because the capture targeted 127.0.0.1 and rustls omits SNI for
// IP literals; production traffic to cli-chat-proxy.grok.com includes it.
func GrokBuildProfile() *Profile {
	return &Profile{
		Name: GrokBuildProfileName,
		CipherSuites: []uint16{
			0x1302, // TLS_AES_256_GCM_SHA384
			0x1301, // TLS_AES_128_GCM_SHA256
			0x1303, // TLS_CHACHA20_POLY1305_SHA256
			0xc02c, // TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
			0xc02b, // TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
			0xcca9, // TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256
			0xc030, // TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
			0xc02f, // TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
			0xcca8, // TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
			0x00ff, // TLS_EMPTY_RENEGOTIATION_INFO_SCSV
		},
		Curves: []uint16{
			0x001d, // X25519
			0x0017, // secp256r1
			0x0018, // secp384r1
		},
		PointFormats: []uint16{0},
		SignatureAlgorithms: []uint16{
			0x0503, // ecdsa_secp384r1_sha384
			0x0403, // ecdsa_secp256r1_sha256
			0x0807, // ed25519
			0x0806, // rsa_pss_pss_sha512
			0x0805, // rsa_pss_pss_sha384
			0x0804, // rsa_pss_pss_sha256
			0x0601, // rsa_pkcs1_sha512
			0x0501, // rsa_pkcs1_sha384
			0x0401, // rsa_pkcs1_sha256
		},
		ALPNProtocols:     []string{"h2", "http/1.1"},
		SupportedVersions: []uint16{0x0304, 0x0303}, // TLS 1.3, TLS 1.2
		KeyShareGroups:    []uint16{0x001d},         // X25519 only
		PSKModes:          []uint16{1},              // psk_dhe_ke
		EnableGREASE:      false,
		Extensions: []uint16{
			0,  // server_name
			10, // supported_groups
			51, // key_share
			45, // psk_key_exchange_modes
			35, // session_ticket
			43, // supported_versions
			11, // ec_point_formats
			16, // alpn
			23, // extended_master_secret
			5,  // status_request
			13, // signature_algorithms
		},
	}
}

// WantsHTTP2 reports whether the profile advertises HTTP/2 via ALPN.
func WantsHTTP2(profile *Profile) bool {
	if profile == nil {
		return false
	}
	for _, proto := range profile.ALPNProtocols {
		if proto == "h2" {
			return true
		}
	}
	return false
}
