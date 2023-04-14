package basstls

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"github.com/gofrs/flock"
	"github.com/square/certstrap/depot"
	"github.com/square/certstrap/pkix"
)

const (
	// Common name for the certificate authority.
	CAName = "bass"

	// Arbitrary values for the CA cert.
	CACountry  = "CA" // Canada (coincidence)
	CAProvince = "Ontario"
	CALocality = "Toronto"

	// RSA key bits.
	keySize = 2048

	// File stored within the cert depot to synchronize cert generation.
	lockFile = "certs.lock"
)

var (
	// DefaultDir is the canonical location to store certs on the user's
	// local machine.
	DefaultDir = filepath.Join(xdg.ConfigHome, "bass", "tls")
)

var CAFiles = []string{CAName + ".crt", CAName + ".key"}

// CACert returns the path to the CA certificate in the given dir.
func CACert(dir string) string {
	return filepath.Join(dir, CAName+".crt")
}

func lockDepot(dir string) (*flock.Flock, error) {
	lock := flock.New(filepath.Join(dir, lockFile))

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	if err := lock.Lock(); err != nil {
		return nil, err
	}

	return lock, nil
}

func getOrClearExpiredCert(d depot.Depot, host string) (*pkix.Certificate, error) {
	crt, err := depot.GetCertificate(d, host)
	if err != nil {
		// no cert; ignore
		return nil, nil
	}

	// cert expired or is expiring soon; remove it
	if crt.GetExpirationDuration() < time.Hour {
		if err := depot.DeleteCertificate(d, host); err != nil {
			return nil, fmt.Errorf("delete expiring cert: %w", err)
		}

		return nil, nil
	}

	// cert exists and is not expiring soon
	return crt, nil
}

// Init initializes dir with a CA.
func Init(dir string) error {
	lock, err := lockDepot(dir)
	if err != nil {
		return err
	}

	defer lock.Unlock()

	d, err := depot.NewFileDepot(dir)
	if err != nil {
		return fmt.Errorf("init depot: %w", err)
	}

	crt, err := getOrClearExpiredCert(d, CAName)
	if err != nil {
		return err
	}

	if crt != nil {
		return nil
	}

	key, err := depot.GetPrivateKey(d, CAName)
	if err != nil {
		key, err = pkix.CreateRSAKey(keySize)
		if err != nil {
			return fmt.Errorf("create key: %w", err)
		}

		err = depot.PutPrivateKey(d, CAName, key)
		if err != nil {
			return fmt.Errorf("put ca: %w", err)
		}
	}

	// TODO(vito): rotate rather than adding a ridiculous amount of time?
	expiry := time.Now().AddDate(0, 0, 64)

	crt, err = pkix.CreateCertificateAuthority(
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

	err = depot.PutCertificate(d, CAName, crt)
	if err != nil {
		return fmt.Errorf("put ca: %w", err)
	}

	return lock.Unlock()
}

func Generate(dir, host string) (*pkix.Certificate, *pkix.Key, error) {
	lock, err := lockDepot(dir)
	if err != nil {
		return nil, nil, err
	}

	defer lock.Unlock()

	d, err := depot.NewFileDepot(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("init depot: %w", err)
	}

	crt, err := getOrClearExpiredCert(d, host)
	if err != nil {
		return nil, nil, err
	}

	if crt != nil {
		key, err := depot.GetPrivateKey(d, host)
		if err != nil {
			return nil, nil, fmt.Errorf("get key for existing cert: %w", err)
		}

		return crt, key, nil
	}

	caCrt, err := depot.GetCertificate(d, CAName)
	if err != nil {
		return nil, nil, fmt.Errorf("get ca cert: %w", err)
	}

	caKey, err := depot.GetPrivateKey(d, CAName)
	if err != nil {
		return nil, nil, fmt.Errorf("get ca key: %w", err)
	}

	key, err := depot.GetPrivateKey(d, host)
	if err != nil {
		key, err = pkix.CreateRSAKey(keySize)
		if err != nil {
			return nil, nil, fmt.Errorf("create key: %w", err)
		}

		err = depot.PutPrivateKey(d, host, key)
		if err != nil {
			return nil, nil, fmt.Errorf("put cert: %w", err)
		}
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

	expiry := time.Now().AddDate(0, 0, 64)

	crt, err = pkix.CreateCertificateHost(caCrt, caKey, csr, expiry)
	if err != nil {
		return nil, nil, fmt.Errorf("create cert: %w", err)
	}

	err = depot.PutCertificate(d, host, crt)
	if err != nil {
		return nil, nil, fmt.Errorf("put cert: %w", err)
	}

	return crt, key, nil
}
