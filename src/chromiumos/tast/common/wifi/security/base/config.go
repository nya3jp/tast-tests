// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package base provides a Config type for open network.
package base

import "chromiumos/tast/common/wifi/security"

// Config implements security.Config interface for open network, i.e., no security.
type Config struct{}

// Static check: Config implements security.Config interface.
var _ security.Config = (*Config)(nil)

// Class returns the security class of open network.
func (*Config) Class() string {
	return "none"
}

// HostapdConfig returns hostapd config of open network.
func (*Config) HostapdConfig() (map[string]string, error) {
	return nil, nil
}

// ShillServiceProperties returns shill properties of open network.
func (*Config) ShillServiceProperties() (map[string]interface{}, error) {
	return nil, nil
}
