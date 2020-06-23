// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package base provides a Config type for open network.
package base

import (
	"context"

	"chromiumos/tast/common/tpmstore"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/local/shill"
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
	return shill.SecurityNone
}

// HostapdConfig returns hostapd config of open network.
func (*Config) HostapdConfig() (map[string]string, error) {
	return nil, nil
}

// ShillServiceProperties returns shill properties of open network.
func (*Config) ShillServiceProperties() (map[string]interface{}, error) {
	return nil, nil
}

// NeedsTPMStore tells that TPMStore is not necessary for this configuration.
func (*Config) NeedsTPMStore() bool {
	return false
}

// InstallRouterCredentials installs the necessary credentials onto router.
func (*Config) InstallRouterCredentials(context.Context, *ssh.Conn, string) error {
	return nil
}

// InstallClientCredentials installs the necessary credentials onto DUT.
func (*Config) InstallClientCredentials(context.Context, *tpmstore.TPMStore) error {
	return nil
}
