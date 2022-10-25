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
		Func:         DataThroughputUdpCallboxServer,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Asserts that cellular data throughput on udp is as expected. The test establishes a connection to the appropriate CMW500 callbox. Then it asserts throughput values and iperf run in server mode on the callbox. Any differences are considered an error.",
		Contacts: []string{
			"srikanthkumar@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:         []string{"group:cellular", "cellular_callbox"},
		ServiceDeps:  []string{"tast.cros.cellular.RemoteCellularService"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "callboxManagedFixture",
		Timeout:      5 * time.Minute,
	})
}

func DataThroughputUdpCallboxServer(ctx context.Context, s *testing.State) {
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
	s.Log("ethernet result: ", ethernetResultStr)
	s.Log("cellular result: ", cellularResultStr )
	if ethernetResultStr != cellularResultStr {
		s.Fatal("Ethernet and cellular curl output not equal")
	}
	// Throughput assertion

	// Setup iperf in server mode on Callbox

	// Run iperf in client mode on DUT to measure UDP throughput for 30 seconds

	// Validate against expected values based on modem model

	// Read modem type
	labels, err := cellular.GetLabelsAsStringArray(ctx, s.Var, "autotest_host_info_labels")
	if err != nil {
		s.Fatal("Failed to read autotest_host_info_labels: ", err)
	}

	helper, err := cellular.NewHelperWithLabels(ctx, labels)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}
	// Get throughput values based on modem type, have a util function

}
