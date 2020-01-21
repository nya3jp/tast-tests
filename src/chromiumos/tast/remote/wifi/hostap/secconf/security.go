// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package secconf

// SecurityConfig defines methods to generate hostapd/shill config of WPA/WEP etc.
type SecurityConfig interface {
	GetClass() string
	GetHostapdConfig() (map[string]string, error)
	GetShillServiceProperties() map[string]interface{}
}

// BaseConfig implements noop methods and can be used in case of open network.
type BaseConfig struct{}

var _ SecurityConfig = (*BaseConfig)(nil)

// GetClass returns security class of open network.
func (*BaseConfig) GetClass() string {
	return "none"
}

// GetHostapdConfig returns hostapd config of open network.
func (*BaseConfig) GetHostapdConfig() (map[string]string, error) {
	return nil, nil
}

// GetShillServiceProperties returns shill properties of open network.
func (*BaseConfig) GetShillServiceProperties() map[string]interface{} {
	return nil
}
