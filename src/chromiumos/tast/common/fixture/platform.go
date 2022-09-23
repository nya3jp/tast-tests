// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

// Fixtures defined in src/chromiumos/tast/remote/bundles/cros/platform/services_on_boot_fixt.go
const (
	// ServicesOnBoot is a fixture for the local platform.ServiceOnBoot.* tests.
	// This fixture reboots the device and waits until services are started.
	// DO NOT USE this fixture on other tests.
	ServicesOnBoot = "servicesOnBootFixt"
)
