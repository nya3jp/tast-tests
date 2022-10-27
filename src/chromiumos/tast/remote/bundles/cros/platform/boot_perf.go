// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	empty "github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/perf"
	tdreq "chromiumos/tast/common/testdevicerequirements"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

const (
	reconnectDelay = 5 * time.Second
)

var (
	defaultIterations      = 10    // The number of boot iterations. Can be overridden by var "platform.BootPerf.iterations".
	defaultSkipRootfsCheck = false // Should we skip rootfs verification? Can be overridden by var "platform.BootPerf.skipRootfsCheck"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BootPerf,
		// The test reboots to the login screen and doesn't require a lacaros variant.
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Boot performance test",
		Contacts:     []string{"chinglinyu@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		ServiceDeps:  []string{"tast.cros.arc.PerfBootService", "tast.cros.platform.BootPerfService", "tast.cros.security.BootLockboxService"},
		// Deps of "chrome" is used to ensure the test doesn't boot to the OOBE screen.
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"platform.BootPerf.iterations", "platform.BootPerf.skipRootfsCheck"},
		// This test collects boot timing for |iterations| times and requires a longer timeout.
		Timeout: 25 * time.Minute,

		// List of requirements this test satisfies.
		Requirements: []string{tdreq.BootPerfKernel, tdreq.BootPerfLogin},
	})
}

// assertRootfsVerification asserts rootfs verification is enabled by
// "checking dm_verity.dev_wait=1" is in /proc/cmdline. Fail the test if rootfs
// verification is disabled.
func assertRootfsVerification(ctx context.Context, s *testing.State) {
	d := s.DUT()
	cmdline, err := d.Conn().CommandContext(ctx, "cat", "/proc/cmdline").Output()
	if err != nil {
		s.Fatal("Failed to read kernel cmdline")
	}

	if !strings.Contains(string(cmdline), "dm_verity.dev_wait=1") {
		s.Fatal("Rootfs verification is off")
	}
}

// preReboot performs actions before rebooting the DUT:
//   - Wait until the CPU is cool.
//   - Stop tlsdated.
func preReboot(ctx context.Context, s *testing.State) {
	// Use a timeout of 30 seconds for waiting until the CPU cools down. A longer wait only has a marginal effect.
	shortCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	arcPerfBootService := arc.NewPerfBootServiceClient(cl.Conn)
	// Wait until CPU cools down with shortCtx.
	if _, err = arcPerfBootService.WaitUntilCPUCoolDown(shortCtx, &empty.Empty{}); err != nil {
		// DUT is unable to cool down, probably timed out. Treat this as a non-fatal error and continue the test with a warning.
		s.Log("Warning: PerfBootService.WaitUntilCPUCoolDown returned an error: ", err)
	}

	bootPerfService := platform.NewBootPerfServiceClient(cl.Conn)
	if _, err = bootPerfService.EnsureTlsdatedStopped(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to stop tlsdated: ", err)
	}
}

// bootPerfOnce runs one iteration of the boot perf test.
func bootPerfOnce(ctx context.Context, s *testing.State, i, iterations int, pv *perf.Values) {
	s.Logf("Running iteration %d/%d", i+1, iterations)
	d := s.DUT()

	preReboot(ctx, s)

	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Wait for |reconnectDelay| duration before reconnecting to the DUT to avoid interfere with early boot stages.
	if err := testing.Sleep(ctx, reconnectDelay); err != nil {
		s.Log("Warning: failed in sleep before redialing RPC: ", err)
	}
	// Need to reconnect to the gRPC server after rebooting DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
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
}

// ensureChromeLogin performs a Chrome login to bypass OOBE if necessary to make
// sure the DUT will be booted to the login screen.
func ensureChromeLogin(ctx context.Context, s *testing.State, cl *rpc.Client) error {
	d := s.DUT()
	// Check whether OOBE is completed.
	if err := d.Conn().CommandContext(ctx, "/usr/bin/test", "-e", "/home/chronos/.oobe_completed").Run(); err == nil {
		return nil
	}

	// Perform a Chrome login to skip OOBE.
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}

	if _, err := client.CloseChrome(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to close Chrome")
	}

	return nil
}

// BootPerf is the function that reboots the client and collect boot perf data.
func BootPerf(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Parse test options.
	skipRootfsCheck := defaultSkipRootfsCheck
	// Check whether the runner requests the test to skip rootfs check.
	if val, ok := s.Var("platform.BootPerf.skipRootfsCheck"); ok {
		// We only accept "true" (case insensitive) as valid value to enable this option. Other values are just ignored silently.
		skipRootfsCheck = (strings.ToLower(val) == "true")
	}

	iterations := defaultIterations
	if iter, ok := s.Var("platform.BootPerf.iterations"); ok {
		if i, err := strconv.Atoi(iter); err == nil {
			iterations = i
		} else {
			// User might want to override the default value of iterations but passed a malformed value. Fail the test to inform the user.
			s.Fatal("Invalid platform.BootPerf.iterations value: ", iter)
		}
	}

	// Create a shorter ctx for normal operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if !skipRootfsCheck {
		// Disabling rootfs verification makes metric "seconds_kernel_to_startup" incorrectly better than normal.
		// This will fail the test if rootfs verification is disabled.
		assertRootfsVerification(ctx, s)
	}

	func(ctx context.Context) {
		// Connect to the gRPC server on the DUT.
		cl, err := rpc.Dial(ctx, d, s.RPCHint())
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		// Make sure we don't boot to OOBE.
		if err = ensureChromeLogin(ctx, s, cl); err != nil {
			s.Fatal("Failed in Chrome login: ", err)
		}

		// Enable bootchart before running the boot perf test.
		bootPerfService := platform.NewBootPerfServiceClient(cl.Conn)
		_, err = bootPerfService.EnableBootchart(ctx, &empty.Empty{})
		if err != nil {
			// If we failed in enabling bootchart, log the failure and proceed without bootchart.
			s.Log("Warning: failed to enable bootchart. Error: ", err)
		}
	}(ctx)

	// Undo the effect of enabling bootchart. This cleanup can also be performed (becomes a no-op) if bootchart is not enabled.
	// Enabling bootchart is persistent (adding an arg to kernel cmdline). Use cleanupCtx to ensure that we have time to undo the effect.
	defer func(ctx context.Context) {
		// Restore the side effect made in this test by disabling bootchart for subsequent system boots.
		s.Log("Disable bootchart")
		cl, err := rpc.Dial(ctx, d, s.RPCHint())
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		bootPerfService := platform.NewBootPerfServiceClient(cl.Conn)
		_, err = bootPerfService.DisableBootchart(ctx, &empty.Empty{})
		if err != nil {
			s.Log("Error in disabling bootchart: ", err)
		}
		// Disabling bootchart will take effect on next boot. Since there is no side effect other than "cros_bootchart" in the kernel cmdline, we skip this reboot.
	}(cleanupCtx)

	pv := perf.NewValues()
	for i := 0; i < iterations; i++ {
		// Run the boot test once.
		bootPerfOnce(ctx, s, i, iterations, pv)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
