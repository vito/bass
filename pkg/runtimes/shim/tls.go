package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	BassCAFile = "/bass/ca.crt"
)

var trusts = map[string][]string{
	"/usr/local/share/ca-certificates/%s.crt": {
		"update-ca-certificates",
	},
	"/usr/share/pki/trust/anchors/%s.pem": {
		"update-ca-certificates",
	},
	"/etc/pki/ca-trust/source/anchors/%s.pem": {
		"update-ca-trust", "extract",
	},
	"/etc/ca-certificates/trust-source/anchors/%s.crt": {
		"trust", "extract-compat",
	},
}

var bundles = []string{
	// alpine, ubuntu
	"/etc/ssl/certs/ca-certificates.crt",
	// nixery
	"/etc/ssl/certs/ca-bundle.crt",
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func binaryExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func installCert() error {
	cert, err := os.ReadFile(BassCAFile)
	if err != nil {
		if os.IsNotExist(err) {
			// NB: certs might not be installed, intentionally
			return nil
		}

		return fmt.Errorf("read bass CA: %w", err)
	}

	// first try to install gracefully to the system trust
	for pathTemplate, cmd := range trusts {
		if _, err := os.Stat(filepath.Dir(pathTemplate)); err != nil {
			continue
		}

		trustPath := fmt.Sprintf(pathTemplate, "bass")
		if err := os.WriteFile(trustPath, cert, 0600); err != nil {
			return fmt.Errorf("write bass CA to system trust: %w", err)
		}

		if _, err := exec.LookPath(cmd[0]); err != nil {
			// installer not found; fall back on injecting
			break
		}

		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install cert: %w\n\noutput:\n\n%s", err, string(out))
		}

		return nil
	}

	// if that doesn't work, try to inject into the bundle
	return injectCert()
}

func injectCert() error {
	for _, path := range bundles {
		if _, err := os.Stat(path); err != nil {
			continue
		}

		bundle, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}

		cert, err := os.Open(BassCAFile)
		if err != nil {
			return err
		}

		if _, err := bundle.Seek(0, os.SEEK_END); err != nil {
			return err
		}

		if _, err := bundle.WriteString("\n"); err != nil {
			return err
		}

		if _, err := io.Copy(bundle, cert); err != nil {
			return err
		}

		return nil
	}

	// this is best-effort

	return nil
}
