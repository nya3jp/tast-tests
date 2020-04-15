// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	empty "github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/local/perf"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

const (
	iterations = 10 // Default to run 10 boot iterations.
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        BootPerf,
		Desc:        "Boot performance test",
		Contacts:    []string{"chinglinyu@chromium.org"},
		Attr:        []string{"group:mainline", "informational"},
		ServiceDeps: []string{"tast.cros.arc.PerfBootService", "tast.cros.platform.BootPerfService"},
		// This test collects boot timing for |iterations| times and requires a longer timeout.
		Timeout: 15 * time.Minute,
	})
}

// assertRootfsVerification asserts rootfs verification is enabled by
// "checking dm_verity.dev_wait=1" is in /proc/cmdline. Fail the test if rootfs
// verification is disabled.
func assertRootfsVerification(ctx context.Context, s *testing.State) {
	d := s.DUT()
	cmdline, err := d.Command("cat", "/proc/cmdline").Output(ctx)
	if err != nil {
		s.Fatal("Failed to read kernel cmdline")
	}

	if strings.Index(string(cmdline), "dm_verity.dev_wait=1") == -1 {
		s.Fatal("Rootfs verification is off")
	}
}

// runOnce runs one iteration of the boot perf test.
func runOnce(ctx context.Context, s *testing.State, i int, pv *perf.Values) {
	s.Logf("Running iteration %d/%d", i+1, iterations)

	d := s.DUT()
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Need to reconnect to the gRPC server after rebooting DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	bootPerfService := platform.NewBootPerfServiceClient(cl.Conn)
	// Collect boot metrics through RPC call to BootPerfServiceClient. This call waits until system boot is complete and returns the metrics.
	metrics, err := bootPerfService.GetBootPerfMetrics(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get boot perf metrics: ", err)
	}

	for k, v := range metrics.GetMetrics() {
		// |unit|: rdbytes or seconds.
		unit := strings.Split(k, "_")[0]
		pv.Append(perf.Metric{
			Name:      k,
			Unit:      unit,
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, v)
	}

	// Save raw data for this iteration.
	savedRaw := filepath.Join(s.OutDir(), fmt.Sprintf("raw.%03d", i+1))
	if err = os.Mkdir(savedRaw, 0755); err != nil {
		s.Fatalf("Failed to create path %s", savedRaw)
	}

	raw, err := bootPerfService.GetBootPerfRawData(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get boot perf raw data: ", err)
	}

	for k, v := range raw.GetRawData() {
		if err = ioutil.WriteFile(filepath.Join(savedRaw, k), v, 0644); err != nil {
			s.Fatal("Failed to save raw data: ", err)
		}
	}

	arcPerfBootService := arc.NewPerfBootServiceClient(cl.Conn)
	// Wait until CPU cool down.
	if _, err = arcPerfBootService.WaitUntilCPUCoolDown(ctx, &empty.Empty{}); err != nil {
		// DUT is unable to cool down, proabaly timed out. Treat this as a non-fatal error and continue the test with a warning.
		s.Log("Warning: PerfBootService.WaitUntilCPUCoolDown returned an error: ", err)
	}

}

// BootPerf is the function that reboots the client and collect boot perf data.
func BootPerf(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Disabling rootfs verification makes metric "seconds_kernel_to_startup" incorrectly better than normal.
	// Fail the test if rootfs verification is disabled.
	assertRootfsVerification(ctx, s)

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Enable bootchart before running the boot perf test.
	bootPerfService := platform.NewBootPerfServiceClient(cl.Conn)
	_, err = bootPerfService.EnableBootchart(ctx, &empty.Empty{})
	if err != nil {
		// If we failed in enabling bootchart, log the failure and proceed without bootchart.
		s.Log("Warning: failed to enable bootchart. Error: ", err)
	}

	pv := perf.NewValues()
	for i := 0; i < iterations; i++ {
		// Run the boot test once.
		runOnce(ctx, s, i, pv)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}

	// Restore the side effect made in this test by disabling bootchart for subsequent system boots.
	s.Log("Disable bootchart")
	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	bootPerfService = platform.NewBootPerfServiceClient(cl.Conn)
	_, err = bootPerfService.DisableBootchart(ctx, &empty.Empty{})
	if err != nil {
		s.Log("Error in disabling bootchart: ", err)
	}
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed in rebooting the device: ", err)
	}
}
