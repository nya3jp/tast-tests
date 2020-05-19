// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dynamicwep

import "chromiumos/tast/common/wifi/security/eap"

// Option is the function signature used to specify options of Config.
type Option func(*ConfigFactory)

// RekeyPeriod returns an Option which sets the rekey period in second in Config.
func RekeyPeriod(period int) Option {
	return func(c *ConfigFactory) {
		c.blueprint.rekeyPeriod = period
	}
}

// UseShortKey returns an Option which sets that we are using short key in Config.
func UseShortKey() Option {
	return func(c *ConfigFactory) {
		c.blueprint.useShortKey = true
	}
}

// Options below are re-wrapped from the options of eap.

// FileSuffix returns an Option which sets the file suffix in Config.
func FileSuffix(file string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.FileSuffix(file))
	}
}

// ClientCACert returns an Option which sets the PEM encoded CA certificate for client in Config.
func ClientCACert(caCert string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.ClientCACert(caCert))
	}
}

// ClientCert returns an Option which sets the PEM encoded identity certificate for client in Config.
func ClientCert(cert string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.ClientCert(cert))
	}
}

// ClientKey returns an Option which sets the PEM encoded private key for client in Config.
func ClientKey(key string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.ClientKey(key))
	}
}

// ClientCertID returns an Option which sets the identifier for client certificate in TPM in Config.
func ClientCertID(certID string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.ClientCertID(certID))
	}
}

// ClientKeyID returns an Option which sets the identifier for client private key in TPM in Config.
func ClientKeyID(keyID string) Option {
	return func(c *ConfigFactory) {
		c.eapOps = append(c.eapOps, eap.ClientKeyID(keyID))
	}
}
