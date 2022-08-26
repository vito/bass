package basstls

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"github.com/square/certstrap/depot"
	"github.com/square/certstrap/pkix"
)

const (
	// Common name for the certificate authority.
	CAName = "bass"

	CACountry  = "CA"
	CAProvince = "Ontario"
	CALocality = "Toronto"

	// RSA key bits.
	keySize = 2048
)

var (
	Dir    = filepath.Join(xdg.ConfigHome, "bass", "tls")
	CACert = filepath.Join(Dir, CAName+".crt")
)

// Init initializes dir with a CA.
func Init() error {
	d, err := depot.NewFileDepot(Dir)
	if err != nil {
		return fmt.Errorf("init depot: %w", err)
	}

	_, err = depot.GetCertificate(d, CAName)
	if err == nil {
		// cert already exists
		return nil
	}

	key, err := pkix.CreateRSAKey(keySize)
	if err != nil {
		return fmt.Errorf("create key: %w", err)
	}

	// TODO(vito): rotate rather than adding a ridiculous amount of time?
	expiry := time.Now().AddDate(0, 0, 64)

	crt, err := pkix.CreateCertificateAuthority(
		key,
		"",
		expiry,
		"",
		CACountry,
		CAProvince,
		CALocality,
		CAName,
		nil,
	)
	if err != nil {
		return fmt.Errorf("create ca: %w", err)
	}

	err = depot.PutPrivateKey(d, CAName, key)
	if err != nil {
		return fmt.Errorf("put ca: %w", err)
	}

	err = depot.PutCertificate(d, CAName, crt)
	if err != nil {
		return fmt.Errorf("put ca: %w", err)
	}

	return nil
}

func Generate(host string) (*pkix.Certificate, *pkix.Key, error) {
	d, err := depot.NewFileDepot(Dir)
	if err != nil {
		return nil, nil, fmt.Errorf("init depot: %w", err)
	}

	caCrt, err := depot.GetCertificate(d, CAName)
	if err != nil {
		return nil, nil, fmt.Errorf("get cert: %w", err)
	}

	caKey, err := depot.GetPrivateKey(d, CAName)
	if err != nil {
		return nil, nil, fmt.Errorf("get key: %w", err)
	}

	key, err := pkix.CreateRSAKey(keySize)
	if err != nil {
		return nil, nil, fmt.Errorf("create key: %w", err)
	}

	csr, err := pkix.CreateCertificateSigningRequest(
		key,
		"",             // Organizational Unit
		nil,            // IPs
		[]string{host}, // Domains
		nil,            // URLs
		"",             // Organization
		CACountry,
		CAProvince,
		CALocality,
		host,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create csr: %w", err)
	}

	// TODO(vito): rotate rather than adding a ridiculous amount of time?
	expiry := time.Now().AddDate(0, 0, 64)

	crt, err := pkix.CreateCertificateHost(caCrt, caKey, csr, expiry)
	if err != nil {
		return nil, nil, fmt.Errorf("create cert: %w", err)
	}

	return crt, key, nil
}
