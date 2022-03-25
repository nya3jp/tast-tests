// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package base provides a Config type for open network.
package base

import (
	"context"

	"chromiumos/tast/common/pkcs11/netcertstore"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/ssh"
)

// Config implements security.Config interface for open network, i.e., no security.
type Config struct{}

// Static check: Config implements security.Config interface.
var _ security.Config = (*Config)(nil)

// ConfigFactory provides Gen method to build a new Config.
type ConfigFactory struct{}

// Gen builds a Config.
func (*ConfigFactory) Gen() (security.Config, error) {
	return &Config{}, nil
}

// NewConfigFactory builds a ConfigFactory.
func NewConfigFactory() *ConfigFactory {
	return &ConfigFactory{}
}

// Static check: ConfigFactory implements security.ConfigFactory interface.
var _ security.ConfigFactory = (*ConfigFactory)(nil)

// Class returns the security class of open network.
func (*Config) Class() string {
	return shillconst.SecurityClassNone
}

// HostapdConfig returns hostapd config of open network.
func (*Config) HostapdConfig() (map[string]string, error) {
	return nil, nil
}

// ShillServiceProperties returns shill properties of open network.
func (*Config) ShillServiceProperties() (map[string]interface{}, error) {
	return nil, nil
}

// NeedsNetCertStore tells that netcert store is not necessary for this configuration.
func (*Config) NeedsNetCertStore() bool {
	return false
}

// InstallRouterCredentials installs the necessary credentials onto router.
func (*Config) InstallRouterCredentials(context.Context, *ssh.Conn, string) error {
	return nil
}

// InstallClientCredentials installs the necessary credentials onto DUT.
func (*Config) InstallClientCredentials(context.Context, *netcertstore.Store) error {
	return nil
}
