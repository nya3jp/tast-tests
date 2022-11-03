// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	cbiperf "chromiumos/tast/remote/cellular/callbox/iperf"
	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataThroughputUDPCallboxServer,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Asserts that cellular data throughput on udp is as expected. The test establishes a connection to the appropriate CMW500 callbox. Then iperf run in server mode on the callbox. Any differences are considered an error",
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

// DataThroughputUDPCallboxServer measures modem throughput with Iperf in server mode on callbox.
func DataThroughputUDPCallboxServer(ctx context.Context, s *testing.State) {
	testURL := "google.com"
	dutConn := s.DUT().Conn()
	tf := s.FixtValue().(*manager.TestFixture)
	if err := tf.ConnectToCallbox(ctx, dutConn, &manager.ConfigureCallboxRequestBody{
		Hardware:     "CMW",
		CellularType: "LTE",
		ParameterList: []string{
			"band", "2",
			"bw", "20",
			"mimo", "2x2", //croscheck mapping
			"tm", "3", //croscheck
			"pul", "0",
			"pdl", "high", //croscheck
		},
	}); err != nil {
		s.Fatal("Failed to initialize cellular connection: ", err)
	}
	testType := cbiperf.TestTypeUDPTx

	// =============================
	// To be removed, Keep this test temporariliy till throughput measurement done on callbox - Start
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
	s.Log("cellular result: ", cellularResultStr)
	if ethernetResultStr != cellularResultStr {
		s.Fatal("Ethernet and cellular curl output not equal")
	}
	// Keep this test temporariliy till throughput measurement done on callbox - End
	// ========================

	// Throughput assertion

	// Setup iperf in server mode on Callbox
	testManager := cbiperf.NewTestManager(tf.Vars.Callbox, dutConn, tf.CallboxManagerClient)

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
	// These options should be set based on modem type of dut read from labels
	additionalOptions := []iperf.ConfigOption{iperf.TestTimeOption(30 * time.Second), iperf.MaxBandwidthOption(100 * iperf.Mbps)}

	// Validate against expected values based on modem model
	history, err := testManager.RunOnce(ctx, testType, tf.InterfaceName, additionalOptions)
	if err != nil {
		s.Fatal("Failed to run iperf session: ", err)
	}
	// Mostly this gets chaned once these ConfigureTxMeasurement, StartTxMeasurement, FetchTxMeasurement,
	// StopTxMeasurement, CloseTxMeasurement implemented

	result, err := iperf.NewResultFromHistory(*history)
	if err != nil {
		s.Fatal("Failed to calculate iperf result statistics: ", err)
	}

	// Run iperf in client mode on DUT to measure UDP throughput for 30 seconds
	min, target, err := testManager.CalculateExpectedThroughput(ctx, testType)
	if err != nil {
		s.Fatal("Failed to calculate expected throughput: ", err)
	}

	if result.Throughput < min {
		s.Fatalf("Throughput below required, got: %v want: %v Mbit/s", result.Throughput/iperf.Mbps, min/iperf.Mbps)
	}

	if result.Throughput < target {
		s.Logf("WARNING: Throughput below target, got: %v want: %v Mbit/s", result.Throughput/iperf.Mbps, target/iperf.Mbps)
	}

	s.Logf("Finished Iperf test %v, minimum: %v target: %v actual: %v +/- %v Mbit/s",
		config.testType, min/iperf.Mbps, target/iperf.Mbps, result.Throughput/iperf.Mbps, result.StdDeviation/iperf.Mbps)

}
