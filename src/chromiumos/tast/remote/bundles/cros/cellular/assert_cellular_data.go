// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssertCellularData,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Asserts that cellular data works. The test establishes a connection to the appropriate CMW500 callbox. Then it asserts that the cellular data connection provided to it matches the data connection provided by ethernet. Any differences are considered an error. If the cellular data connection is not provided, the second curl will throw an exception",
		Contacts: []string{
			"latware@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:         []string{"group:cellular", "cellular_callbox"},
		ServiceDeps:  []string{"tast.cros.cellular.RemoteCellularService"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "callboxManagedFixture",
		Timeout:      5 * time.Minute,
	})
}

func AssertCellularData(ctx context.Context, s *testing.State) {
	testURL := "google.com"
	dutConn := s.DUT().Conn()
	tf := s.FixtValue().(*manager.TestFixture)
	if err := tf.ConnectToCallbox(ctx, dutConn, &manager.ConfigureCallboxRequestBody{
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
		s.Fatal("Failed to initialize cellular connection: ", err)
	}

	// Assert cellular connection on DUT can connect to a URL like ethernet can
	ethernetResult, err := dutConn.CommandContext(ctx, "curl", "--interface", "eth0", testURL).Output()
	if err != nil {
		s.Fatalf("Failed to curl %q on DUT using ethernet interface: %v", testURL, err)
	}

	cellularResult, err := dutConn.CommandContext(ctx, "curl", "--interface", tf.InterfaceName, testURL).Output()
	if err != nil {
		s.Fatalf("Failed to curl %q on DUT using cellular interface: %v", testURL, err)
	}
	ethernetResultStr := string(ethernetResult)
	cellularResultStr := string(cellularResult)
	if ethernetResultStr != cellularResultStr {
		s.Fatal("Ethernet and cellular curl output not equal")
	}
}
