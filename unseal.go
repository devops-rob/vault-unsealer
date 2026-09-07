package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	logger "github.com/sirupsen/logrus"
)

// SealStatus captures the seal status response from Vault.
type SealStatus struct {
	Sealed bool `json:"sealed"`
}

// UnsealRequest is the payload for a single unseal key submission.
type UnsealRequest struct {
	Key string `json:"key"`
}

// requestTimeout bounds every individual HTTP call to a Vault node.
const requestTimeout = 15 * time.Second

// newVaultClient builds an HTTP client for talking to Vault. It enforces TLS
// verification by default (system trust store, or an explicit CA bundle) and
// applies a request timeout so a hung node cannot stall the probe loop.
func newVaultClient(cfg TLSConfig) (*http.Client, error) {
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.SkipVerify, //nolint:gosec // opt-in, guarded and logged below
	}

	if cfg.SkipVerify {
		logger.Warn("TLS certificate verification is DISABLED (tls.skip_verify=true); do not use this outside local testing")
	}

	if cfg.CACert != "" {
		pem, err := os.ReadFile(cfg.CACert)
		if err != nil {
			return nil, fmt.Errorf("reading tls.ca_cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("tls.ca_cert %q contained no valid PEM certificates", cfg.CACert)
		}
		tlsCfg.RootCAs = pool
	}

	transport := &http.Transport{TLSClientConfig: tlsCfg}
	return &http.Client{Timeout: requestTimeout, Transport: transport}, nil
}

// sealStatus reports whether the given Vault node is currently sealed.
func sealStatus(ctx context.Context, client *http.Client, server string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server+"/v1/sys/seal-status", nil)
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	var status SealStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return false, fmt.Errorf("decoding seal-status response: %w", err)
	}
	return status.Sealed, nil
}

// submitUnsealKey submits a single unseal key and returns whether Vault is still
// sealed afterwards.
func submitUnsealKey(ctx context.Context, client *http.Client, server, key string) (bool, error) {
	payload, err := json.Marshal(UnsealRequest{Key: key})
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/v1/sys/unseal", bytes.NewReader(payload))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	var status SealStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return false, fmt.Errorf("decoding unseal response: %w", err)
	}
	return status.Sealed, nil
}

// checkAndUnsealVault probes a single Vault node and, if it is sealed, fetches
// unseal keys from the provider and submits them until the node is unsealed.
// Keys are only requested from the provider when an unseal is actually needed.
func checkAndUnsealVault(ctx context.Context, client *http.Client, server string, provider KeyProvider) {
	sealed, err := sealStatus(ctx, client, server)
	if err != nil {
		logger.Errorf("%s: error fetching seal status: %v", server, err)
		return
	}
	if !sealed {
		logger.Infof("%s is already unsealed.", server)
		return
	}

	logger.Infof("%s is sealed. Attempting to unseal...", server)

	keys, err := provider.UnsealKeys(ctx)
	if err != nil {
		logger.Errorf("%s: could not retrieve unseal keys: %v", server, err)
		return
	}

	for _, key := range keys {
		stillSealed, err := submitUnsealKey(ctx, client, server, key)
		if err != nil {
			logger.Errorf("%s: error submitting unseal key: %v", server, err)
			return
		}
		if !stillSealed {
			logger.Infof("%s is now unsealed.", server)
			return
		}
	}

	logger.Warnf("%s is still sealed after submitting all available keys.", server)
}

// monitorAndUnsealVaults probes every configured node on an interval and unseals
// any that are sealed, until the context is cancelled.
func monitorAndUnsealVaults(ctx context.Context, client *http.Client, servers []string, provider KeyProvider, probeInterval int) {
	ticker := time.NewTicker(time.Duration(probeInterval) * time.Second)
	defer ticker.Stop()

	for {
		for _, server := range servers {
			go checkAndUnsealVault(ctx, client, server, provider)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
