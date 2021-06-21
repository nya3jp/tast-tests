// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package security defines interfaces for test authors to deal with the
// diverse WiFi security standards:
// - Interface Config defines methods to compose configuration files of AP or DUT.
// - Interface ConfigFactory lets the authors to register the security options
//   in testing.AddTest and Gen the Config object later in test body.
package security

import (
	sec "chromiumos/tast/common/wifi/security"
	"chromiumos/tast/remote/wificell/router"
)

// AxConfig defines methods to generate hostapd and shill config of protected network.
type AxConfig interface {
	RouterParams() []router.AxRouterConfigParam
	SecConfig() sec.Config
}

// ConfigFactory defines a Gen() method to generate a Config instance.
// A security protocol should implement its own ConfigFactory interface as well as a NewConfigFactory()
// function to compose a declarative factory object for generating a security config.
// A factory, once created via NewConfigFactory, holds a list of options provided in testing.AddTest.
// Gen() then uses the stored options to compose a Config.
// Noted that the creation of a ConfigFactory object must not emit an error to satisfy the
// requirement of declarative test registration.
type AxConfigFactory interface {
	Gen() (AxConfig, error)
}
