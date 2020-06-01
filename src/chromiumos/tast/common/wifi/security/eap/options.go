// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package eap

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

// ClientCert returns an Option which sets the PEM encoded identity certificate for client in Config.
func ClientCert(cert string) Option {
	return func(c *Config) {
		c.clientCert = cert
	}
}

// ClientKey returns an Option which sets the PEM encoded private key for client in Config.
func ClientKey(key string) Option {
	return func(c *Config) {
		c.clientKey = key
	}
}

// TPMID returns an Option which sets the identifier for client cert/key in TPM in Config.
func TPMID(id string) Option {
	return func(c *Config) {
		c.tpmID = id
	}
}
