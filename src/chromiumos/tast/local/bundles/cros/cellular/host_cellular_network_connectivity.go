// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HostCellularNetworkConnectivity,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that host has network connectivity via cellular interface",
		Contacts:     []string{"madhavadas@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Timeout:      1 * time.Minute,
	})
}

func HostCellularNetworkConnectivity(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime*2)
	defer cancel()

	enableEthernetFunc, enableWifiFunc, err := helper.SetupCellularInterfaceForTesting(ctx)
	if err != nil {
		s.Fatal("Failed to setup cellular interface for testing: ", err)
	}
	if enableEthernetFunc != nil {
		defer enableEthernetFunc(cleanupCtx)
	}
	if enableWifiFunc != nil {
		defer enableWifiFunc(cleanupCtx)
	}

	ipType, err := helper.GetApnIPType(ctx)
	if err != nil {
		s.Fatal("Failed to read APN info: ", err)
	}
	if err := cellular.VerifyIPConnectivity(ctx, testexec.CommandContext, ipType, "/bin"); err != nil {
		s.Error("Failed connectivity test : ", err)
	}
}
