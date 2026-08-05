package http

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

var (
	ErrInvalidCert      = errors.New("invalid certificate")
	ErrInvalidKey       = errors.New("invalid private key")
	ErrCertKeyMismatch  = errors.New("certificate and private key mismatch")
	ErrInvalidCA        = errors.New("invalid CA certificate")
	ErrMissingCertOrKey = errors.New("both certificate and key must be provided")
)

type TLSConfig struct {
	CertFile              string
	KeyFile               string
	CertData              []byte
	KeyData               []byte
	CAFile                string
	CAData                []byte
	InsecureSkipVerify    bool
	ServerName            string
	MinVersion            uint16
	MaxVersion            uint16
	CipherSuites          []uint16
	CurvePreferences      []tls.CurveID
	ClientAuth            tls.ClientAuthType
	RootCAs               *x509.CertPool
	ClientCAs             *x509.CertPool
	VerifyPeerCertificate func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error
	VerifyConnection      func(cs tls.ConnectionState) error
}

func NewTLSConfig() *TLSConfig {
	return &TLSConfig{
		MinVersion:       tls.VersionTLS12,
		MaxVersion:       tls.VersionTLS13,
		ClientAuth:       tls.NoClientCert,
		CipherSuites:     defaultCipherSuites(),
		CurvePreferences: defaultCurvePreferences(),
	}
}

func defaultCipherSuites() []uint16 {
	return []uint16{
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}
}

func defaultCurvePreferences() []tls.CurveID {
	return []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
		tls.CurveP384,
		tls.CurveP521,
	}
}

func (tc *TLSConfig) LoadCertKeyFromFile(certFile, keyFile string) error {
	certData, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCert, err)
	}

	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}

	tc.CertFile = certFile
	tc.KeyFile = keyFile
	tc.CertData = certData
	tc.KeyData = keyData

	return nil
}

func (tc *TLSConfig) LoadCAFromFile(caFile string) error {
	caData, err := os.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCA, err)
	}

	tc.CAFile = caFile
	tc.CAData = caData
	return nil
}

func (tc *TLSConfig) SetCertKeyData(certData, keyData []byte) error {
	if len(certData) == 0 || len(keyData) == 0 {
		return ErrMissingCertOrKey
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return ErrInvalidCert
	}

	block, _ = pem.Decode(keyData)
	if block == nil {
		return ErrInvalidKey
	}

	tc.CertData = certData
	tc.KeyData = keyData
	return nil
}

func (tc *TLSConfig) SetCAData(caData []byte) error {
	if len(caData) == 0 {
		return ErrInvalidCA
	}

	block, _ := pem.Decode(caData)
	if block == nil {
		return ErrInvalidCA
	}

	tc.CAData = caData
	return nil
}

func (tc *TLSConfig) BuildTLSConfig() (*tls.Config, error) {
	config := &tls.Config{
		InsecureSkipVerify:    tc.InsecureSkipVerify,
		ServerName:            tc.ServerName,
		MinVersion:            tc.MinVersion,
		MaxVersion:            tc.MaxVersion,
		CipherSuites:          tc.CipherSuites,
		CurvePreferences:      tc.CurvePreferences,
		ClientAuth:            tc.ClientAuth,
		RootCAs:               tc.RootCAs,
		ClientCAs:             tc.ClientCAs,
		VerifyPeerCertificate: tc.VerifyPeerCertificate,
		VerifyConnection:      tc.VerifyConnection,
	}

	if len(tc.CertData) > 0 && len(tc.KeyData) > 0 {
		cert, err := tls.X509KeyPair(tc.CertData, tc.KeyData)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCertKeyMismatch, err)
		}
		config.Certificates = []tls.Certificate{cert}
	} else if tc.CertFile != "" && tc.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCertKeyMismatch, err)
		}
		config.Certificates = []tls.Certificate{cert}
	}

	if len(tc.CAData) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(tc.CAData) {
			return nil, ErrInvalidCA
		}
		config.RootCAs = pool
	} else if tc.CAFile != "" {
		caData, err := os.ReadFile(tc.CAFile)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidCA, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caData) {
			return nil, ErrInvalidCA
		}
		config.RootCAs = pool
	}

	if tc.RootCAs != nil {
		config.RootCAs = tc.RootCAs
	}

	if tc.ClientCAs != nil {
		config.ClientCAs = tc.ClientCAs
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return config, nil
}

func (tc *TLSConfig) BuildClientTLSConfig() (*tls.Config, error) {
	config, err := tc.BuildTLSConfig()
	if err != nil {
		return nil, err
	}

	config.ClientAuth = tls.NoClientCert
	return config, nil
}

func (tc *TLSConfig) BuildServerTLSConfig() (*tls.Config, error) {
	config, err := tc.BuildTLSConfig()
	if err != nil {
		return nil, err
	}

	if len(config.Certificates) == 0 {
		return nil, ErrMissingCertOrKey
	}

	return config, nil
}

func (tc *TLSConfig) WithMutualTLS(caData []byte) *TLSConfig {
	tc.ClientAuth = tls.RequireAndVerifyClientCert
	_ = tc.SetCAData(caData)
	return tc
}

func (tc *TLSConfig) WithInsecureSkipVerify(skip bool) *TLSConfig {
	tc.InsecureSkipVerify = skip
	return tc
}

func (tc *TLSConfig) WithServerName(name string) *TLSConfig {
	tc.ServerName = name
	return tc
}

func (tc *TLSConfig) WithMinVersion(version uint16) *TLSConfig {
	tc.MinVersion = version
	return tc
}

func (tc *TLSConfig) WithCipherSuites(suites []uint16) *TLSConfig {
	tc.CipherSuites = suites
	return tc
}

func ParseCertificate(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrInvalidCert
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCert, err)
	}

	return cert, nil
}

func ParsePrivateKey(pemData []byte) (interface{}, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, ErrInvalidKey
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	return nil, ErrInvalidKey
}

func CertificateToPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

func PrivateKeyToPEM(key interface{}) ([]byte, error) {
	var pkcs8Key []byte
	var err error

	switch k := key.(type) {
	case *rsa.PrivateKey:
		pkcs8Key, err = x509.MarshalPKCS8PrivateKey(k)
	case *ecdsa.PrivateKey:
		pkcs8Key, err = x509.MarshalPKCS8PrivateKey(k)
	default:
		return nil, ErrInvalidKey
	}

	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Key,
	}), nil
}

func LoadSystemCertPool() (*x509.CertPool, error) {
	return x509.SystemCertPool()
}

func NewCertPool(certs ...*x509.Certificate) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AddCert(cert)
	}
	return pool
}

func (tc *TLSConfig) Clone() *TLSConfig {
	return &TLSConfig{
		CertFile:              tc.CertFile,
		KeyFile:               tc.KeyFile,
		CertData:              tc.CertData,
		KeyData:               tc.KeyData,
		CAFile:                tc.CAFile,
		CAData:                tc.CAData,
		InsecureSkipVerify:    tc.InsecureSkipVerify,
		ServerName:            tc.ServerName,
		MinVersion:            tc.MinVersion,
		MaxVersion:            tc.MaxVersion,
		CipherSuites:          tc.CipherSuites,
		CurvePreferences:      tc.CurvePreferences,
		ClientAuth:            tc.ClientAuth,
		RootCAs:               tc.RootCAs,
		ClientCAs:             tc.ClientCAs,
		VerifyPeerCertificate: tc.VerifyPeerCertificate,
		VerifyConnection:      tc.VerifyConnection,
	}
}
