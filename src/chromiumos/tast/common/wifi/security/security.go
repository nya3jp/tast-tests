// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package security defines interfaces for test authors to deal with the
// diverse WiFi security standards:
// - Interface Config defines methods to get config of AP or DUT.
// - Interface Factory lets the authors to register the security options
//   in testing.AddTest and Gen the Config object later in test body.
package security

// Config defines methods to generate hostapd and shill config of protected network.
type Config interface {
	// Class returns the corresponding shill SecurityClass properties of the WiFi service.
	Class() string
	// HostapdConfig returns hostapd config of the WiFi service.
	HostapdConfig() (map[string]string, error)
	// ShillServiceProperties returns the shill properties that the DUT should set in
	// order to connect to the WiFi service configured by HostapdConfig.
	ShillServiceProperties() (map[string]interface{}, error)
}

// Factory defines Gen method.
// A security protocol should implement its own Factory interface as well as a NewFactory()
// function to compose a declarative factory object for generating a security config.
// A factory, once created via NewFactory, holds a list of options provided in
// testing.AddTest. Gen() then uses the stored options to compose a Config.
// Noted that the creation of an Factory object must not emit an error to satisfy the
// requirement of declarative test registration.
type Factory interface {
	Gen() (Config, error)
}
