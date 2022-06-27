// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package passpoint

import (
	"strconv"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
)

const (
	testUser        = "test-user"
	testPassword    = "test-password"
	testPackageName = "app.passpoint.example.com"
)

// Auth is the authentication method the access point will expose.
type Auth int

const (
	// AuthTLS represents the EAP-TLS authentication method.
	AuthTLS Auth = iota
	// AuthTTLS represents the EAP-TTLS with MSCHAPv2 authentication method.
	AuthTTLS
)

// Credentials represents a set of Passpoint credentials with selection criteria.
type Credentials struct {
	// Domain represents the domain of the service provider.
	Domain string
	// HomeOI is a list of organisation identifiers (OI).
	HomeOIs []uint64
	// RequiredHomeOIs is a list of required organisation identifiers.
	RequiredHomeOIs []uint64
	// RoamingOIs is a list of roaming compatible OIs.
	RoamingOIs []uint64
	// Auth is the EAP network authentication.
	Auth
}

// ToShillProperties converts the set of credentials to a map for credentials D-Bus
// properties. ToShillProperties only supports EAP-TTLS authentication.
func (pc *Credentials) ToShillProperties() (map[string]interface{}, error) {
	if pc.Auth != AuthTTLS {
		return nil, errors.Errorf("unsupported authentication method: %v", pc.Auth)
	}

	certs := certificate.TestCert1()
	props := map[string]interface{}{
		shillconst.PasspointCredentialsPropertyDomains:            []string{pc.Domain},
		shillconst.PasspointCredentialsPropertyRealm:              pc.Domain,
		shillconst.PasspointCredentialsPropertyMeteredOverride:    false,
		shillconst.PasspointCredentialsPropertyAndroidPackageName: testPackageName,
		shillconst.ServicePropertyEAPMethod:                       "TTLS",
		shillconst.ServicePropertyEAPInnerEAP:                     "auth=MSCHAPV2",
		shillconst.ServicePropertyEAPIdentity:                     testUser,
		shillconst.ServicePropertyEAPPassword:                     testPassword,
		shillconst.ServicePropertyEAPCACertPEM:                    []string{certs.CACred.Cert},
	}

	for propName, ois := range map[string][]uint64{
		shillconst.PasspointCredentialsPropertyHomeOIs:          pc.HomeOIs,
		shillconst.PasspointCredentialsPropertyRequiredHomeOIs:  pc.RequiredHomeOIs,
		shillconst.PasspointCredentialsPropertyRoamingConsortia: pc.RoamingOIs,
	} {
		var propOIs []string
		for _, oi := range ois {
			propOIs = append(propOIs, strconv.FormatUint(oi, 10))
		}
		props[propName] = propOIs
	}

	return props, nil
}
