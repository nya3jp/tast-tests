// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package security defines interfaces for test authors to deal with the
// diverse WiFi security standards:
// - Interface Config defines methods to compose configuration files of AP or DUT.
// - Interface ConfigFactory lets the authors to register the security options
//   in testing.AddTest and Gen the Config object later in test body.
package security

import (
	"context"

	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/dut"
	"chromiumos/tast/ssh"
)

// Config defines methods to generate hostapd and shill config of protected network.
type Config interface {
	// Class returns the SecurityClass (defined in shill/service.go) of the WiFi service
	// which is used for searching for WiFi service.
	Class() string
	// HostapdConfig returns the hostapd config of the WiFi service.
	// Note that InstallRouterCredentials() may update the router config so that it should be
	// called before HostapdConfig(). Also, the implementation shall perform sanity check.
	HostapdConfig() (map[string]string, error)
	// ShillServiceProperties returns the shill properties that the DUT should set in
	// order to connect to the WiFi service configured by HostapdConfig.
	// Note that InstallClientCredentials() may update the client config so that it should be
	// called before ShillServiceProperties(). Also, the implementation shall perform sanity check.
	ShillServiceProperties() (map[string]interface{}, error)
	// InstallRouterCredentials installs the nacessary credentials onto router.
	InstallRouterCredentials(ctx context.Context, host *ssh.Conn, workDir string) error
	// InstallClientCredentials installs the nacessary credentials onto DUT.
	InstallClientCredentials(context.Context, *pkcs11.Chaps, *dut.DUT) error
}

// ConfigFactory defines a Gen() method to generate a Config instance.
// A security protocol should implement its own ConfigFactory interface as well as a NewConfigFactory()
// function to compose a declarative factory object for generating a security config.
// A factory, once created via NewConfigFactory, holds a list of options provided in testing.AddTest.
// Gen() then uses the stored options to compose a Config.
// Noted that the creation of a ConfigFactory object must not emit an error to satisfy the
// requirement of declarative test registration.
type ConfigFactory interface {
	Gen() (Config, error)
}
