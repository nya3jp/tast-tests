// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	localnearby "chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/nearbyshare"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultiDUTUISmoke,
		Desc:         "Checks we can enable Nearby Share high-vis receving on two DUTs at once",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.nearbyshare.NearbyShareService"},
		Vars:         []string{"secondaryTarget"},
	})
}

// MultiDUTUISmoke tests that we can enable Nearby Share on two DUTs in a single test.
func MultiDUTUISmoke(ctx context.Context, s *testing.State) {
	// TODO(b/175889133) Remove hardcoded hostnames when multi dut skylab support is available.
	const (
		HatchHostname   = "chromeos15-row6a-rack12-host2a.cros"
		OctopusHostname = "chromeos15-row6a-rack12-host2b.cros"
	)
	d1 := s.DUT()

	// Figure out which DUT is primary and which is secondary.
	// Switch on the DUTs in our lab setup first, then fall back to user supplied var.
	var secondaryDUT string
	if strings.Contains(s.DUT().HostName(), HatchHostname) {
		secondaryDUT = OctopusHostname
	} else if strings.Contains(s.DUT().HostName(), OctopusHostname) {
		secondaryDUT = HatchHostname
	} else {
		secondary, ok := s.Var("secondaryTarget")
		if !ok {
			s.Fatal("Test is running on an unknown hostname and no secondaryTarget arg was supplied")
		}
		secondaryDUT = secondary
	}

	s.Log("Connecting to secondary DUT: ", secondaryDUT)
	d2, err := d1.NewSecondaryDevice(secondaryDUT)
	if err != nil {
		s.Fatal("Failed to create secondary device: ", err)
	}
	if err := d2.Connect(ctx); err != nil {
		s.Fatal("Failed to connect to secondary DUT: ", err)
	}

	if err := openHighVisibilityMode(ctx, s, d1); err != nil {
		s.Fatal("Failed to enable high vis mode on primary DUT: ", err)
	}

	if err := openHighVisibilityMode(ctx, s, d2); err != nil {
		s.Fatal("Failed to enable high vis mode on secondary DUT: ", err)
	}
}

// openHighVisibilityMode is a helper function to enable high vis mode on each DUT.
func openHighVisibilityMode(ctx context.Context, s *testing.State, d *dut.DUT) error {
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)

	// Connect to the Nearby Share Service so we can execute local code on the DUT.
	ns := nearbyshare.NewNearbyShareServiceClient(cl.Conn)
	if _, err := ns.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer ns.CloseChrome(ctx, &empty.Empty{})

	// Setup Nearby Share on the DUT.
	const deviceName = "MultiDut_HighVisibilityUISmoke"
	req := &nearbyshare.CrOSSetupRequest{DataUsage: int32(localnearby.DataUsageOnline), Visibility: int32(localnearby.VisibilityAllContacts), DeviceName: deviceName}
	if _, err := ns.CrOSSetup(ctx, req); err != nil {
		s.Fatal("Failed to setup Nearby Share: ", err)
	}

	// Enable high visibility mode.
	if _, err := ns.StartHighVisibilityMode(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start High Visibility mode: ", err)
	}
	return nil
}
