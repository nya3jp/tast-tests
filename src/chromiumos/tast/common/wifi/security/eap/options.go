// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package eap

import "chromiumos/tast/common/crypto/certificate"

// Option is the function signature used to specify options of Config.
type Option func(*Config)

// FileSuffix returns an Option which sets the file suffix in Config.
func FileSuffix(suffix string) Option {
	return func(c *Config) {
		c.fileSuffix = suffix
	}
}

// Identity returns an Option which sets the user to authenticate as in Config.
func Identity(id string) Option {
	return func(c *Config) {
		c.identity = id
	}
}

// ServerEAPUsers returns an Option which sets the EAP users for server in Config.
func ServerEAPUsers(users string) Option {
	return func(c *Config) {
		c.serverEAPUsers = users
	}
}

// ClientCACert returns an Option which sets the PEM encoded CA certificate for client in Config.
func ClientCACert(caCert string) Option {
	return func(c *Config) {
		c.clientCACert = caCert
	}
}

// ClientCred returns an Option which sets the PEM encoded credentials for client in Config.
func ClientCred(cred certificate.Credential) Option {
	return func(c *Config) {
		c.clientCred = cred
	}
}

// TPMID returns an Option which sets the identifier for client cert/key in TPM in Config.
func TPMID(id string) Option {
	return func(c *Config) {
		c.tpmID = id
	}
}
