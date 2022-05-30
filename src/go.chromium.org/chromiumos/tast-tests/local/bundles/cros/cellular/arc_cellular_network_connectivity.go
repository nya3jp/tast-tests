// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/local/arc"
	"go.chromium.org/chromiumos/tast-tests/local/cellular"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/modemmanager"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcCellularNetworkConnectivity,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that Arc has network connectivity via cellular interface",
		Contacts:     []string{"madhavadas@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Fixture:      "cellular",
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ArcCellularNetworkConnectivity(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// StartARC
	func() {
		cr, err := chrome.New(ctx, chrome.ARCEnabled())
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		defer cr.Close(ctx)
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close(ctx)
	}()

	ipType, err := helper.GetCurrentIPType(ctx)
	if err != nil {
		s.Fatal("Failed to read APN info: ", err)
	}

	verifyIPConnectivity := func(ctx context.Context) error {
		if err := cellular.VerifyIPConnectivity(ctx, arc.BootstrapCommand, ipType, "/system/bin"); err != nil {
			return errors.Wrap(err, "failed connectivity test")
		}
		return nil
	}
	if err := helper.RunTestOnCellularInterface(ctx, verifyIPConnectivity); err != nil {
		s.Fatal("Failed to run test on cellular interface: ", err)
	}
}
