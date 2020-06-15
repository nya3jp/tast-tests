// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"math/rand"
	"os"

	"chromiumos/tast/common/crypto/certificate"
)

func main() {
	// TODO: With https://github.com/golang/go/issues/21915, some crypto
	// operation we use is not deterministic and will generate different
	// keys/certs everytime. This makes the program not very suitable for
	// codegen (e.g. introduce unnecessary diff and affect existing tests)
	f := NewFactory()
	cert, err := f.Gen(rand.New(rand.NewSource(0)))
	if err != nil {
		fmt.Println("Error generating certificates:", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", pretty(*cert))
}

// pretty formats the certificate object into string which is more suitable
// for user to copy paste the output.
// TODO: Now here are some pieces related to the certificate format in
// different places. e.g. prettyPrint here, formatter of Factory, unit test of
// certificates. We should find some way to merge them so that we won't forget
// to update any of them when the behavior changes.
func pretty(cert certificate.Certificate) string {
	// TODO: better format this.
	return fmt.Sprintf("%#v", cert)
}

// TODO: The value of CACert1 from Autotest. Remove this later.
/*
var caCertRef = x509.Certificate{
	SignatureAlgorithm: 3,
	PublicKeyAlgorithm: 1,
	PublicKey:          (*rsa.PublicKey)(0xc00000d140),
	Version:            3,
	SerialNumber:       15708327353576505672,
	Issuer: pkix.Name{
		Country:            []string{"US"},
		Organization:       []string(nil),
		OrganizationalUnit: []string(nil),
		Locality:           []string{"Mountain View"},
		Province:           []string{"California"},
		StreetAddress:      []string(nil),
		PostalCode:         []string(nil),
		SerialNumber:       "",
		CommonName:         "chromelab-wifi-testbed-root.mtv.google.com",
		Names:              []pkix.AttributeTypeAndValue{pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "US"}, pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 8}, Value: "California"}, pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 7}, Value: "Mountain View"}, pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "chromelab-wifi-testbed-root.mtv.google.com"}}, ExtraNames: []pkix.AttributeTypeAndValue(nil)},
	Subject: pkix.Name{
		Country:            []string{"US"},
		Organization:       []string(nil),
		OrganizationalUnit: []string(nil),
		Locality:           []string{"Mountain View"},
		Province:           []string{"California"}, StreetAddress: []string(nil), PostalCode: []string(nil), SerialNumber: "",
		CommonName: "chromelab-wifi-testbed-root.mtv.google.com",
		Names: []pkix.AttributeTypeAndValue{
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "US"},
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 8}, Value: "California"},
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 7}, Value: "Mountain View"},
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "chromelab-wifi-testbed-root.mtv.google.com"},
		},
		ExtraNames: []pkix.AttributeTypeAndValue(nil),
	},
	NotBefore: time.Time{wall: 0x0, ext: 63471001771, loc: (*time.Location)(nil)},
	NotAfter:  time.Time{wall: 0x0, ext: 63786361771, loc: (*time.Location)(nil)},
	KeyUsage:  0,
	Extensions: []pkix.Extension{
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 14}, Critical: false, Value: []uint8{0x4, 0x14, 0x32, 0x67, 0x21, 0x8d, 0x91, 0x8b, 0xca, 0xe3, 0xd2, 0x5f, 0x56, 0x23, 0xea, 0xe9, 0xca, 0xb3, 0xf9, 0xac, 0x94, 0x3f}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 35}, Critical: false, Value: []uint8{0x30, 0x81, 0x96, 0x80, 0x14, 0x32, 0x67, 0x21, 0x8d, 0x91, 0x8b, 0xca, 0xe3, 0xd2, 0x5f, 0x56, 0x23, 0xea, 0xe9, 0xca, 0xb3, 0xf9, 0xac, 0x94, 0x3f, 0xa1, 0x73, 0xa4, 0x71, 0x30, 0x6f, 0x31, 0xb, 0x30, 0x9, 0x6, 0x3, 0x55, 0x4, 0x6, 0x13, 0x2, 0x55, 0x53, 0x31, 0x13, 0x30, 0x11, 0x6, 0x3, 0x55, 0x4, 0x8, 0x13, 0xa, 0x43, 0x61, 0x6c, 0x69, 0x66, 0x6f, 0x72, 0x6e, 0x69, 0x61, 0x31, 0x16, 0x30, 0x14, 0x6, 0x3, 0x55, 0x4, 0x7, 0x13, 0xd, 0x4d, 0x6f, 0x75, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x20, 0x56, 0x69, 0x65, 0x77, 0x31, 0x33, 0x30, 0x31, 0x6, 0x3, 0x55, 0x4, 0x3, 0x13, 0x2a, 0x63, 0x68, 0x72, 0x6f, 0x6d, 0x65, 0x6c, 0x61, 0x62, 0x2d, 0x77, 0x69, 0x66, 0x69, 0x2d, 0x74, 0x65, 0x73, 0x74, 0x62, 0x65, 0x64, 0x2d, 0x72, 0x6f, 0x6f, 0x74, 0x2e, 0x6d, 0x74, 0x76, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x63, 0x6f, 0x6d, 0x82, 0x9, 0x0, 0xd9, 0xff, 0x30, 0x80, 0x75, 0x7a, 0xc1, 0x48}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 19}, Critical: false, Value: []uint8{0x30, 0x3, 0x1, 0x1, 0xff}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: false, Value: []uint8{0x30, 0xa, 0x6, 0x8, 0x2b, 0x6, 0x1, 0x5, 0x5, 0x7, 0x3, 0x3}},
	},
	ExtraExtensions:             []pkix.Extension(nil),
	UnhandledCriticalExtensions: []asn1.ObjectIdentifier(nil),
	ExtKeyUsage:                 []x509.ExtKeyUsage{3},
	UnknownExtKeyUsage:          []asn1.ObjectIdentifier(nil),
	BasicConstraintsValid:       true,
	IsCA:                        true,
	MaxPathLen:                  -1,
	MaxPathLenZero:              false,
	SubjectKeyId:                []uint8{0x32, 0x67, 0x21, 0x8d, 0x91, 0x8b, 0xca, 0xe3, 0xd2, 0x5f, 0x56, 0x23, 0xea, 0xe9, 0xca, 0xb3, 0xf9, 0xac, 0x94, 0x3f},
	AuthorityKeyId:              []uint8{0x32, 0x67, 0x21, 0x8d, 0x91, 0x8b, 0xca, 0xe3, 0xd2, 0x5f, 0x56, 0x23, 0xea, 0xe9, 0xca, 0xb3, 0xf9, 0xac, 0x94, 0x3f},
	OCSPServer:                  []string(nil),
	IssuingCertificateURL:       []string(nil),
	DNSNames:                    []string(nil),
	EmailAddresses:              []string(nil),
	IPAddresses:                 []net.IP(nil),
	URIs:                        []*url.URL(nil),
	PermittedDNSDomainsCritical: false,
	PermittedDNSDomains:         []string(nil),
	ExcludedDNSDomains:          []string(nil),
	PermittedIPRanges:           []*net.IPNet(nil),
	ExcludedIPRanges:            []*net.IPNet(nil),
	PermittedEmailAddresses:     []string(nil),
	ExcludedEmailAddresses:      []string(nil),
	PermittedURIDomains:         []string(nil),
	ExcludedURIDomains:          []string(nil),
	CRLDistributionPoints:       []string(nil),
	PolicyIdentifiers:           []asn1.ObjectIdentifier(nil),
}
*/
/*
var serverCert = &x509.Certificate{
	SignatureAlgorithm: 2,
	PublicKeyAlgorithm: 1,
	PublicKey:          (*rsa.PublicKey)(0xc000210720),
	Version:            3,
	SerialNumber:       1048579,
	Issuer: pkix.Name{
		Country:            []string{"US"},
		Organization:       []string(nil),
		OrganizationalUnit: []string(nil),
		Locality:           []string{"Mountain View"},
		Province:           []string{"California"},
		StreetAddress:      []string(nil),
		PostalCode:         []string(nil),
		SerialNumber:       "",
		CommonName:         "chromelab-wifi-testbed-root.mtv.google.com",
		Names: []pkix.AttributeTypeAndValue{
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "US"},
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 8}, Value: "California"},
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 7}, Value: "Mountain View"},
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "chromelab-wifi-testbed-root.mtv.google.com"},
		},
		ExtraNames: []pkix.AttributeTypeAndValue(nil),
	},
	Subject: pkix.Name{
		Country:            []string{"US"},
		Organization:       []string(nil),
		OrganizationalUnit: []string(nil),
		Locality:           []string{"Mountain View"},
		Province:           []string{"California"},
		StreetAddress:      []string(nil),
		PostalCode:         []string(nil),
		SerialNumber:       "",
		CommonName:         "chromelab-wifi-testbed-server.mtv.google.com",
		Names: []pkix.AttributeTypeAndValue{
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "US"},
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 8}, Value: "California"},
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 7}, Value: "Mountain View"},
			pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "chromelab-wifi-testbed-server.mtv.google.com"},
		},
		ExtraNames: []pkix.AttributeTypeAndValue(nil),
	},
	NotBefore: time.Time{wall: 0x0, ext: 63471001775, loc: (*time.Location)(nil)},
	NotAfter:  time.Time{wall: 0x0, ext: 63786361775, loc: (*time.Location)(nil)},
	KeyUsage:  5,
	Extensions: []pkix.Extension{
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 19}, Critical: false, Value: []uint8{0x30, 0x0}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 16, 840, 1, 113730, 1, 1}, Critical: false, Value: []uint8{0x3, 0x2, 0x6, 0x40}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 14}, Critical: false, Value: []uint8{0x4, 0x14, 0x81, 0x60, 0x25, 0xbb, 0x64, 0x57, 0xda, 0x18, 0x9a, 0x53, 0xe2, 0xae, 0x39, 0xd2, 0xf1, 0x8f, 0x2, 0xf1, 0x7b, 0x74}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 35}, Critical: false, Value: []uint8{0x30, 0x81, 0x96, 0x80, 0x14, 0x32, 0x67, 0x21, 0x8d, 0x91, 0x8b, 0xca, 0xe3, 0xd2, 0x5f, 0x56, 0x23, 0xea, 0xe9, 0xca, 0xb3, 0xf9, 0xac, 0x94, 0x3f, 0xa1, 0x73, 0xa4, 0x71, 0x30, 0x6f, 0x31, 0xb, 0x30, 0x9, 0x6, 0x3, 0x55, 0x4, 0x6, 0x13, 0x2, 0x55, 0x53, 0x31, 0x13, 0x30, 0x11, 0x6, 0x3, 0x55, 0x4, 0x8, 0x13, 0xa, 0x43, 0x61, 0x6c, 0x69, 0x66, 0x6f, 0x72, 0x6e, 0x69, 0x61, 0x31, 0x16, 0x30, 0x14, 0x6, 0x3, 0x55, 0x4, 0x7, 0x13, 0xd, 0x4d, 0x6f, 0x75, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x20, 0x56, 0x69, 0x65, 0x77, 0x31, 0x33, 0x30, 0x31, 0x6, 0x3, 0x55, 0x4, 0x3, 0x13, 0x2a, 0x63, 0x68, 0x72, 0x6f, 0x6d, 0x65, 0x6c, 0x61, 0x62, 0x2d, 0x77, 0x69, 0x66, 0x69, 0x2d, 0x74, 0x65, 0x73, 0x74, 0x62, 0x65, 0x64, 0x2d, 0x72, 0x6f, 0x6f, 0x74, 0x2e, 0x6d, 0x74, 0x76, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x63, 0x6f, 0x6d, 0x82, 0x9, 0x0, 0xd9, 0xff, 0x30, 0x80, 0x75, 0x7a, 0xc1, 0x48}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 15}, Critical: false, Value: []uint8{0x3, 0x2, 0x5, 0xa0}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: false, Value: []uint8{0x30, 0xa, 0x6, 0x8, 0x2b, 0x6, 0x1, 0x5, 0x5, 0x7, 0x3, 0x1}},
	},
	ExtraExtensions:             []pkix.Extension(nil),
	UnhandledCriticalExtensions: []asn1.ObjectIdentifier(nil),
	ExtKeyUsage:                 []x509.ExtKeyUsage{1},
	UnknownExtKeyUsage:          []asn1.ObjectIdentifier(nil),
	BasicConstraintsValid:       true,
	IsCA:                        false,
	MaxPathLen:                  -1,
	MaxPathLenZero:              false,
	SubjectKeyId:                []uint8{0x81, 0x60, 0x25, 0xbb, 0x64, 0x57, 0xda, 0x18, 0x9a, 0x53, 0xe2, 0xae, 0x39, 0xd2, 0xf1, 0x8f, 0x2, 0xf1, 0x7b, 0x74},
	AuthorityKeyId:              []uint8{0x32, 0x67, 0x21, 0x8d, 0x91, 0x8b, 0xca, 0xe3, 0xd2, 0x5f, 0x56, 0x23, 0xea, 0xe9, 0xca, 0xb3, 0xf9, 0xac, 0x94, 0x3f},
	OCSPServer:                  []string(nil), IssuingCertificateURL: []string(nil), DNSNames: []string(nil), EmailAddresses: []string(nil), IPAddresses: []net.IP(nil),
	URIs: []*url.URL(nil), PermittedDNSDomainsCritical: false, PermittedDNSDomains: []string(nil), ExcludedDNSDomains: []string(nil),
	PermittedIPRanges: []*net.IPNet(nil), ExcludedIPRanges: []*net.IPNet(nil), PermittedEmailAddresses: []string(nil), ExcludedEmailAddresses: []string(nil),
	PermittedURIDomains: []string(nil), ExcludedURIDomains: []string(nil), CRLDistributionPoints: []string(nil), PolicyIdentifiers: []asn1.ObjectIdentifier(nil),
}

var clientCert = x509.Certificate{
	SignatureAlgorithm: 2,
	PublicKeyAlgorithm: 1,
	PublicKey:          (*rsa.PublicKey)(0xc000211ae0),
	Version:            3,
	SerialNumber:       1048577,
	Issuer:             pkix.Name{Country: []string{"US"}, Organization: []string(nil), OrganizationalUnit: []string(nil), Locality: []string{"Mountain View"}, Province: []string{"California"}, StreetAddress: []string(nil), PostalCode: []string(nil), SerialNumber: "", CommonName: "chromelab-wifi-testbed-root.mtv.google.com", Names: []pkix.AttributeTypeAndValue{pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "US"}, pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 8}, Value: "California"}, pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 7}, Value: "Mountain View"}, pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "chromelab-wifi-testbed-root.mtv.google.com"}}, ExtraNames: []pkix.AttributeTypeAndValue(nil)},
	Subject:            pkix.Name{Country: []string{"US"}, Organization: []string(nil), OrganizationalUnit: []string(nil), Locality: []string{"Mountain View"}, Province: []string{"California"}, StreetAddress: []string(nil), PostalCode: []string(nil), SerialNumber: "", CommonName: "chromelab-wifi-testbed-client.mtv.google.com", Names: []pkix.AttributeTypeAndValue{pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "US"}, pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 8}, Value: "California"}, pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 7}, Value: "Mountain View"}, pkix.AttributeTypeAndValue{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "chromelab-wifi-testbed-client.mtv.google.com"}}, ExtraNames: []pkix.AttributeTypeAndValue(nil)},
	NotBefore:          time.Time{wall: 0x0, ext: 63471001771, loc: (*time.Location)(nil)},
	NotAfter:           time.Time{wall: 0x0, ext: 63786361771, loc: (*time.Location)(nil)}, KeyUsage: 5,
	Extensions: []pkix.Extension{
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 19}, Critical: false, Value: []uint8{0x30, 0x0}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 14}, Critical: false, Value: []uint8{0x4, 0x14, 0x95, 0x54, 0xdd, 0x8d, 0x57, 0x35, 0xad, 0xc7, 0x24, 0x85, 0x4a, 0x7, 0xdb, 0x68, 0x9a, 0x4f, 0x13, 0xb7, 0xc0, 0x68}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 35}, Critical: false, Value: []uint8{0x30, 0x81, 0x96, 0x80, 0x14, 0x32, 0x67, 0x21, 0x8d, 0x91, 0x8b, 0xca, 0xe3, 0xd2, 0x5f, 0x56, 0x23, 0xea, 0xe9, 0xca, 0xb3, 0xf9, 0xac, 0x94, 0x3f, 0xa1, 0x73, 0xa4, 0x71, 0x30, 0x6f, 0x31, 0xb, 0x30, 0x9, 0x6, 0x3, 0x55, 0x4, 0x6, 0x13, 0x2, 0x55, 0x53, 0x31, 0x13, 0x30, 0x11, 0x6, 0x3, 0x55, 0x4, 0x8, 0x13, 0xa, 0x43, 0x61, 0x6c, 0x69, 0x66, 0x6f, 0x72, 0x6e, 0x69, 0x61, 0x31, 0x16, 0x30, 0x14, 0x6, 0x3, 0x55, 0x4, 0x7, 0x13, 0xd, 0x4d, 0x6f, 0x75, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x20, 0x56, 0x69, 0x65, 0x77, 0x31, 0x33, 0x30, 0x31, 0x6, 0x3, 0x55, 0x4, 0x3, 0x13, 0x2a, 0x63, 0x68, 0x72, 0x6f, 0x6d, 0x65, 0x6c, 0x61, 0x62, 0x2d, 0x77, 0x69, 0x66, 0x69, 0x2d, 0x74, 0x65, 0x73, 0x74, 0x62, 0x65, 0x64, 0x2d, 0x72, 0x6f, 0x6f, 0x74, 0x2e, 0x6d, 0x74, 0x76, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x63, 0x6f, 0x6d, 0x82, 0x9, 0x0, 0xd9, 0xff, 0x30, 0x80, 0x75, 0x7a, 0xc1, 0x48}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 15}, Critical: false, Value: []uint8{0x3, 0x2, 0x5, 0xa0}},
		pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 37}, Critical: false, Value: []uint8{0x30, 0xa, 0x6, 0x8, 0x2b, 0x6, 0x1, 0x5, 0x5, 0x7, 0x3, 0x2}}},
	ExtraExtensions: []pkix.Extension(nil), UnhandledCriticalExtensions: []asn1.ObjectIdentifier(nil), ExtKeyUsage: []x509.ExtKeyUsage{2},
	UnknownExtKeyUsage: []asn1.ObjectIdentifier(nil), BasicConstraintsValid: true, IsCA: false, MaxPathLen: -1, MaxPathLenZero: false,
	SubjectKeyId:   []uint8{0x95, 0x54, 0xdd, 0x8d, 0x57, 0x35, 0xad, 0xc7, 0x24, 0x85, 0x4a, 0x7, 0xdb, 0x68, 0x9a, 0x4f, 0x13, 0xb7, 0xc0, 0x68},
	AuthorityKeyId: []uint8{0x32, 0x67, 0x21, 0x8d, 0x91, 0x8b, 0xca, 0xe3, 0xd2, 0x5f, 0x56, 0x23, 0xea, 0xe9, 0xca, 0xb3, 0xf9, 0xac, 0x94, 0x3f},
	OCSPServer:     []string(nil), IssuingCertificateURL: []string(nil), DNSNames: []string(nil), EmailAddresses: []string(nil), IPAddresses: []net.IP(nil),
	URIs: []*url.URL(nil), PermittedDNSDomainsCritical: false, PermittedDNSDomains: []string(nil), ExcludedDNSDomains: []string(nil),
	PermittedIPRanges: []*net.IPNet(nil), ExcludedIPRanges: []*net.IPNet(nil), PermittedEmailAddresses: []string(nil), ExcludedEmailAddresses: []string(nil),
	PermittedURIDomains: []string(nil), ExcludedURIDomains: []string(nil), CRLDistributionPoints: []string(nil), PolicyIdentifiers: []asn1.ObjectIdentifier(nil),
}
*/
