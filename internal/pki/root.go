//go:build go1.15
// +build go1.15

package pki

/*
 Copyright 2021 Crunchy Data Solutions, Inc.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"time"
)

const (
	// defaultRootCAExpiration sets the default time for the root CA, which is
	// placed far enough into the future
	defaultRootCAExpiration = 10 * 365 * 24 * time.Hour

	// rootCAName is the name of the root CA
	rootCAName = "postgres-operator-ca"
)

// RootCertificateAuthority contains the ability to generate the necessary
// components of a root certificate authority (root CA). This includes the
// private key for the root CA as well as its certificate, which is self-signed
// (as is the nature of a root CA).
//
// In the context of the Operator, there will be one root certificate per
// namespace that contains postgresclusters managed by the Operator.
type RootCertificateAuthority struct {
	// Certificate is the certificate of this certificate authority
	Certificate *Certificate

	// PrivateKey is the private key portion of the certificate authority
	PrivateKey *PrivateKey

	// generateKey generates an ECDSA keypair
	generateKey func() (*ecdsa.PrivateKey, error)

	// generateCertificate generates a X509 certificate return in DER format
	generateCertificate func(*ecdsa.PrivateKey, *big.Int) ([]byte, error)

	// generateSerialNumber creates a unique serial number to assign to the
	// certificate
	generateSerialNumber func() (*big.Int, error)
}

// Generate creates a new root certificate authority
func (ca *RootCertificateAuthority) Generate() error {
	// ensure functions are defined
	if ca.generateKey == nil || ca.generateCertificate == nil || ca.generateSerialNumber == nil {
		return ErrFunctionNotImplemented
	}

	// generate a private key
	if privateKey, err := ca.generateKey(); err != nil {
		return err
	} else {
		ca.PrivateKey = NewPrivateKey(privateKey)
	}

	// generate a serial number
	serialNumber, err := ca.generateSerialNumber()

	if err != nil {
		return err
	}

	// generate a certificate
	if certificate, err := ca.generateCertificate(ca.PrivateKey.PrivateKey, serialNumber); err != nil {
		return err
	} else {
		ca.Certificate = &Certificate{Certificate: certificate}
	}

	return nil
}

// NewRootCertificateAuthority generates a new root certificate authority
// that can be used to issue leaf certificates
func NewRootCertificateAuthority() *RootCertificateAuthority {
	return &RootCertificateAuthority{
		generateCertificate:  generateRootCertificate,
		generateKey:          generateKey,
		generateSerialNumber: generateSerialNumber,
	}
}

// ParseRootCertificateAuthority takes a PEM encoded private key and certificate
// representation and attempts to parse it.
func ParseRootCertificateAuthority(privateKey, certificate []byte) (*RootCertificateAuthority, error) {
	var err error
	ca := NewRootCertificateAuthority()

	// attempt to parse the private key
	if ca.PrivateKey, err = ParsePrivateKey(privateKey); err != nil {
		return nil, err
	}

	// attempt to parse the certificate
	if ca.Certificate, err = ParseCertificate(certificate); err != nil {
		return nil, err
	}

	return ca, nil
}

// RootCAIsBad checks that at least one root CA has been generated and that
// all returned certs are CAs and not expired
//
// TODO(tjmoore4): Currently this will return 'true' if any of the parsed certs
// fail a given check. For scenarios where multiple certs may be returned, such
// as in a BYOC/BYOCA, this will need to be handled so we only generate a new
// certificate for our cert if it is the one that fails.
func RootCAIsBad(root *RootCertificateAuthority) bool {
	// if the certificate or the private key are nil, the root CA is bad
	if root.Certificate == nil || root.PrivateKey == nil {
		return true
	}

	// if there is an error parsing the root certificate or if there is not at least one certificate,
	// the RootCertificateAuthority is bad
	rootCerts, rootErr := x509.ParseCertificates(root.Certificate.Certificate)

	if rootErr != nil && len(rootCerts) < 1 {
		return true
	}

	// find our root cert in the returned slice
	for _, cert := range rootCerts {
		// root cert is bad if it is not a CA
		if !cert.IsCA || !cert.BasicConstraintsValid {
			return true
		}

		// if it is outside of the certs configured valid time range
		if time.Now().After(cert.NotAfter) || time.Now().Before(cert.NotBefore) {
			return true
		}
	}

	// checks passed, cert is good
	return false

}

// generateRootCertificate creates a x509 certificate with a ECDSA signature using
// the SHA-384 algorithm
func generateRootCertificate(privateKey *ecdsa.PrivateKey, serialNumber *big.Int) ([]byte, error) {
	// prepare the certificate. set the validity time to the predefined range
	now := time.Now()
	template := &x509.Certificate{
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		MaxPathLenZero:        true, // there are no intermediate certificates
		NotBefore:             now.Add(beforeInterval),
		NotAfter:              now.Add(defaultRootCAExpiration),
		SerialNumber:          serialNumber,
		SignatureAlgorithm:    certificateSignatureAlgorithm,
		Subject: pkix.Name{
			CommonName: rootCAName,
		},
	}

	// a root certificate has no parent, so pass in the template twice
	return x509.CreateCertificate(rand.Reader, template, template,
		privateKey.Public(), privateKey)
}
