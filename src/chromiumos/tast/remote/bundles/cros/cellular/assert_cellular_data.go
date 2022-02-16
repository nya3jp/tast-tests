// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/exec"
	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AssertCellularData,
		Desc:     "Asserts that cellular data works. The test establishes a connection to the appropriate CMW500 callbox. Then it asserts that the cellular data connection provided to it matches the data connection provided by ethernet. Any differences are considered an error. If the cellular data connection is not provided, the second curl will throw an exception",
		Contacts: []string{
			// None yet
		},
		Attr:         []string{},
		ServiceDeps:  []string{},
		SoftwareDeps: []string{},
		Fixture:      "callboxManagedFixture",
		Timeout:      5 * time.Minute,
	})
}

func AssertCellularData(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*manager.TestFixture)
	dutConn := s.DUT().Conn()

	// Disable and then re-enable cellular on DUT
	if err := dutConn.CommandContext(ctx, "dbus-send", []string{
		"--system",
		"--fixed",
		"--print-reply",
		"--dest=org.chromium.flimflam",
		"/",
		"org.chromium.flimflam.Manager.DisableTechnology",
		"string:cellular",
	}...).Run(exec.DumpLogOnError); err != nil {
		s.Fatal("Failed to disable DUT cellular: ", err)
	}
	if err := dutConn.CommandContext(ctx, "dbus-send", []string{
		"--system",
		"--fixed",
		"--print-reply",
		"--dest=org.chromium.flimflam",
		"/",
		"org.chromium.flimflam.Manager.EnableTechnology",
		"string:cellular",
	}...).Run(exec.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable DUT cellular: ", err)
	}

	// Preform callbox simulation
	if err := tf.CallboxManagerClient.ConfigureCallbox(ctx, &manager.ConfigureCallboxRequestBody{
		Hardware:     "CMW",
		CellularType: "LTE",
		ParameterList: []string{
			"band", "2",
			"bw", "20",
			"mimo", "2x2",
			"tm", "1",
			"pul", "0",
			"pdl", "high",
		},
	}); err != nil {
		s.Fatal("Failed to configure callbox: ", err)
	}
	if err := tf.CallboxManagerClient.BeginSimulation(ctx, nil); err != nil {
		s.Fatal("Failed to begin callbox simulation: ", err)
	}

	// Assert cellular connection on DUT can connect to a URL like ethernet can
	testURL := "google.com"
	ethernetResult, err := dutConn.CommandContext(ctx, "curl", "--interface", "eth0", testURL).Output()
	if err != nil {
		s.Fatalf("Failed to curl %q on DUT using ethernet interface: %v", testURL, err)
	}
	cellularResult, err := dutConn.CommandContext(ctx, "curl", "--interface", "rmnet_data0", testURL).Output()
	if err != nil {
		s.Fatalf("Failed to curl %q on DUT using cellular interface: %v", testURL, err)
	}
	ethernetResultStr := string(ethernetResult)
	cellularResultStr := string(cellularResult)
	if ethernetResultStr != cellularResultStr {
		s.Fatal("Ethernet and cellular curl output not equal")
	}
}
