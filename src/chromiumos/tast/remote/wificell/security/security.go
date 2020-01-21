// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

// Config defines methods to generate hostapd and shill config of protected network.
type Config interface {
	Class() string
	HostapdConfig() (map[string]string, error)
	ShillServiceProperties() map[string]interface{}
}

// Factory defines Gen method.
// The security types should implement the type Factory and the function NewFactory.
// Factory holds the WiFi options that are registered in testing.AddTest, and is
// able to generate a Config struct according to the options by Gen.
// NewFactory builds a Factory without raising errors thus can be used for registering.
type Factory interface {
	Gen() (Config, error)
}
