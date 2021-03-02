// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/remote/bundles/cros/nearbyshare/remotetestutils"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/nearbyshare"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultiDUTUISmoke,
		Desc:         "Checks we can enable Nearby Share high-vis receving on two DUTs at once",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:nearby-share-remote"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.nearbyshare.NearbyShareService"},
		Vars:         []string{"secondaryTarget"},
	})
}

// MultiDUTUISmoke tests that we can enable Nearby Share on two DUTs in a single test.
func MultiDUTUISmoke(ctx context.Context, s *testing.State) {
	d1 := s.DUT()
	secondary, ok := s.Var("secondaryTarget")
	if !ok {
		secondary = ""
	}
	secondaryDUT, err := nearbytestutils.ChooseSecondaryDUT(d1.HostName(), secondary)
	if err != nil {
		s.Fatal("Failed to find hostname for DUT2: ", err)
	}

	s.Log("Connecting to secondary DUT: ", secondaryDUT)
	d2, err := d1.NewSecondaryDevice(secondaryDUT)
	if err != nil {
		s.Fatal("Failed to create secondary device: ", err)
	}
	if err := d2.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to secondary DUT: ", err)
	}

	if err := openHighVisibilityMode(ctx, s, d1, "dut1"); err != nil {
		s.Fatal("Failed to enable high vis mode on primary DUT: ", err)
	}

	if err := openHighVisibilityMode(ctx, s, d2, "dut2"); err != nil {
		s.Fatal("Failed to enable high vis mode on secondary DUT: ", err)
	}
}

// openHighVisibilityMode is a helper function to enable high vis mode on each DUT.
func openHighVisibilityMode(ctx context.Context, s *testing.State, d *dut.DUT, tag string) error {
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)

	// Connect to the Nearby Share Service so we can execute local code on the DUT.
	ns := nearbyshare.NewNearbyShareServiceClient(cl.Conn)
	loginReq := &nearbyshare.CrOSLoginRequest{}
	if _, err := ns.NewChromeLogin(ctx, loginReq); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer remotetestutils.SaveLogs(ctx, ns, d, tag, s.OutDir())

	// Setup Nearby Share on the DUT.
	const deviceName = "MultiDut_HighVisibilityUISmoke"
	req := &nearbyshare.CrOSSetupRequest{DataUsage: int32(nearbysetup.DataUsageOnline), Visibility: int32(nearbysetup.VisibilityAllContacts), DeviceName: deviceName}
	if _, err := ns.CrOSSetup(ctx, req); err != nil {
		s.Fatal("Failed to setup Nearby Share: ", err)
	}

	// Enable high visibility mode.
	if _, err := ns.StartHighVisibilityMode(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start High Visibility mode: ", err)
	}
	return nil
}
