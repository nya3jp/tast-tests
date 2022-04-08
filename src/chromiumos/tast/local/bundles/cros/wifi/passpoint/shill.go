// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package passpoint

import (
	"math/rand"
	"strconv"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
)

const (
	testUser        = "test-user"
	testPassword    = "test-password"
	testPackageName = "app.passpoint.example.com"
)

// Credentials represents a set of Passpoint credentials with selection criteria
type Credentials struct {
	// Domain of the service provider
	Domain string
	// List of organisation identifiers (OI)
	HomeOIs []uint64
	// List of required organisation identifiers
	RequiredHomeOIs []uint64
	// List of roaming compatible OIs
	RoamingOIs []uint64
}

// ToProperties converts the set of credentials to a map for credentials D-Bus
// properties.
func (pc *Credentials) ToProperties() map[string]interface{} {
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

	if len(pc.HomeOIs) > 0 {
		var ois []string
		for _, oi := range pc.HomeOIs {
			ois = append(ois, strconv.FormatUint(oi, 10))
		}
		props[shillconst.PasspointCredentialsPropertyHomeOIs] = ois
	}

	if len(pc.RequiredHomeOIs) > 0 {
		var ois []string
		for _, oi := range pc.RequiredHomeOIs {
			ois = append(ois, strconv.FormatUint(oi, 10))
		}
		props[shillconst.PasspointCredentialsPropertyRequiredHomeOIs] = ois
	}

	if len(pc.RoamingOIs) > 0 {
		var ois []string
		for _, oi := range pc.RoamingOIs {
			ois = append(ois, strconv.FormatUint(oi, 10))
		}
		props[shillconst.PasspointCredentialsPropertyRoamingConsortia] = ois
	}

	return props
}

// RandomProfileName returns a random name for Shill test profile.
func RandomProfileName() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	s := make([]byte, 8)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return "passpoint" + string(s)
}
