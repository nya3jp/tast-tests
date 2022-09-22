// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HostCellularStressEnableDisable,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that host has network connectivity via cellular interface",
		Contacts:     []string{"madhavadas@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture:      "cellular",
		Timeout:      4 * time.Minute,
	})
}

func HostCellularStressEnableDisable(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	stressTestHostIPConnectivity := func(ctx context.Context) error {
		for i := 1; i < 5; i++ {
			s.Logf("Test loop: %d", i)
			s.Log("Disable")
			if _, err := helper.Disable(ctx); err != nil {
				return errors.Wrap(err, "failed to disable modem")
			}
			s.Log("Enable")
			// Enable and get service to set autoconnect based on test parameters.
			if _, err := helper.Enable(ctx); err != nil {
				return errors.Wrap(err, "failed to enable modem")
			}
			ipv4, ipv6, err := helper.GetNetworkProvisionedCellularIPTypes(ctx)
			if err != nil {
				s.Fatal("Failed to read APN info: ", err)
			}
			s.Log("ipv4: ", ipv4, " ipv6: ", ipv6)
			if err := cellular.VerifyIPConnectivity(ctx, testexec.CommandContext, ipv4, ipv6, "/bin"); err != nil {
				return errors.Wrap(err, "failed connectivity test")
			}
		}
		return nil
	}

	if err := helper.RunTestOnCellularInterface(ctx, stressTestHostIPConnectivity); err != nil {
		s.Fatal("Failed to run test on cellular interface: ", err)
	}
}
