// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package base

import "chromiumos/tast/remote/wificell/security"

// Config implements noop methods and can be used in case of open network.
type Config struct{}

var _ security.Config = (*Config)(nil)

// Class returns security class of open network.
func (*Config) Class() string {
	return "none"
}

// HostapdConfig returns hostapd config of open network.
func (*Config) HostapdConfig() (map[string]string, error) {
	return nil, nil
}

// ShillServiceProperties returns shill properties of open network.
func (*Config) ShillServiceProperties() map[string]interface{} {
	return nil
}
