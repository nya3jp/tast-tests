// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strconv"

	"github.com/golang/protobuf/ptypes/empty"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/common/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/nearbyservice"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SmokeMultiDUTUI,
		Desc:         "Checks we can enable Nearby Share high-vis receving on two DUTs at once",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:nearby-share-remote"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.nearbyservice.NearbyShareService"},
		// TODO(crbug/1127165): Move to fixture when data is available in fixtures.
		Data: []string{"small_jpg.zip", "small_png.zip", "big_txt.zip"},
		Vars: []string{nearbycommon.KeepStateVar},
	})
}

// SmokeMultiDUTUI tests that we can enable Nearby Share on two DUTs in a single test.
func SmokeMultiDUTUI(ctx context.Context, s *testing.State) {
	d1 := s.DUT()
	var d2 *dut.DUT
	// Check if there is a hardcoded secondary DUT assigned to the current host.
	secondaryDUT, err := nearbytestutils.ChooseSecondaryDUT(d1.HostName())
	if err == nil {
		s.Log("Ensuring we can connect to DUT2 from the hardcoded pairs: ", secondaryDUT)
		d2, err = d1.NewSecondaryDevice(secondaryDUT)
		if err != nil {
			s.Fatal("Failed to create secondary device: ", err)
		}
		if err := d2.Connect(ctx); err != nil {
			s.Fatal("Failed to connect to secondary DUT: ", err)
		}
	} else {
		s.Log("No secondary DUT found in hardcoded pairs. Checking if a companion DUT was passed")
		d2 = s.CompanionDUT("cd1")
		if d2 == nil {
			s.Fatal("Failed to get companion DUT cd1")
		}
	}

	var keepState bool
	if val, ok := s.Var(nearbycommon.KeepStateVar); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatalf("Unable to convert %v var to bool: %v", nearbycommon.KeepStateVar, err)
		}
		keepState = b
	}
	if err := openHighVisibilityMode(ctx, s, d1, "dut1", keepState); err != nil {
		s.Fatal("Failed to enable high vis mode on primary DUT: ", err)
	}

	if err := openHighVisibilityMode(ctx, s, d2, "dut2", keepState); err != nil {
		s.Fatal("Failed to enable high vis mode on secondary DUT: ", err)
	}
}

// openHighVisibilityMode is a helper function to enable high vis mode on each DUT.
func openHighVisibilityMode(ctx context.Context, s *testing.State, d *dut.DUT, tag string, keepState bool) error {
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)

	// Connect to the Nearby Share Service so we can execute local code on the DUT.
	ns := nearbyservice.NewNearbyShareServiceClient(cl.Conn)
	loginReq := &nearbyservice.CrOSLoginRequest{KeepState: keepState}
	if _, err := ns.NewChromeLogin(ctx, loginReq); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer ns.CloseChrome(ctx, &empty.Empty{})

	// Setup Nearby Share on the DUT.
	const deviceName = "MultiDut_HighVisibilityUISmoke"
	req := &nearbyservice.CrOSSetupRequest{DataUsage: int32(nearbysetup.DataUsageOnline), Visibility: int32(nearbysetup.VisibilityAllContacts), DeviceName: deviceName}
	if _, err := ns.CrOSSetup(ctx, req); err != nil {
		s.Fatal("Failed to setup Nearby Share: ", err)
	}

	// Enable high visibility mode.
	if _, err := ns.StartHighVisibilityMode(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start High Visibility mode: ", err)
	}
	return nil
}
