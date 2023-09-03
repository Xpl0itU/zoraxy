package acme

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"
)

// Get the issuer name from pem file
func ExtractIssuerNameFromPEM(pemFilePath string) (string, error) {
	// Read the PEM file
	pemData, err := os.ReadFile(pemFilePath)
	if err != nil {
		return "", err
	}

	return ExtractIssuerName(pemData)
}

// Get the DNSName in the cert
func ExtractDomains(certBytes []byte) ([]string, error) {
	domains := []string{}
	block, _ := pem.Decode(certBytes)
	if block != nil {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return []string{}, err
		}
		for _, dnsName := range cert.DNSNames {
			if !contains(domains, dnsName) {
				domains = append(domains, dnsName)
			}
		}

		return domains, nil
	}
	return []string{}, errors.New("decode cert bytes failed")
}

func ExtractIssuerName(certBytes []byte) (string, error) {
	// Parse the PEM block
	block, _ := pem.Decode(certBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("failed to decode PEM block containing certificate")
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %v", err)
	}

	// Extract the issuer name
	issuer := cert.Issuer.Organization[0]

	return issuer, nil
}

// Check if a cert is expired by public key
func CertIsExpired(certBytes []byte) bool {
	block, _ := pem.Decode(certBytes)
	if block != nil {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err == nil {
			elapsed := time.Since(cert.NotAfter)
			if elapsed > 0 {
				// if it is expired then add it in
				// make sure it's uniqueless
				return true
			}
		}
	}
	return false
}

func CertExpireSoon(certBytes []byte) bool {
	block, _ := pem.Decode(certBytes)
	if block != nil {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err == nil {
			expirationDate := cert.NotAfter
			threshold := 14 * 24 * time.Hour // 14 days

			timeRemaining := time.Until(expirationDate)
			if timeRemaining <= threshold {
				return true
			}
		}
	}
	return false
}
