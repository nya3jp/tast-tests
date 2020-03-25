// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

// This file defines the common interface for different protocol to implement.

// Config defines methods to generate hostapd and shill config of protected network.
type Config interface {
	GetClass() string
	GetHostapdConfig() (map[string]string, error)
	GetShillServiceProperties() map[string]interface{}
}

// Generator defines Gen method. The security types who provides test options should
// implement this interface and thus be able to register some options in testing.AddTest
// and Gen a Config during test.
type Generator interface {
	Gen() (Config, error)
}
