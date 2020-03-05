// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
	"time"

	"chromiumos/tast/local/network"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HostapHwsim,
		Desc: "Run selected hostap tests using a set of simulated WiFi clients/APs",
		Contacts: []string{
			"briannorris@chromium.org",
			"chromeos-kernel-wifi@google.com",
		},
		Attr: []string{"group:mainline"},

		SoftwareDeps: []string{"hostap_hwsim"},
		// For running manually, with specific 'run-all.sh' arguments (e.g., specific tests or
		// modules).
		Vars: []string{"network.HostapHwsim.runArgs"},

		Params: []testing.Param{
			{
				Name: "sanity",
				// Keep this test list short, as it can take a while to run many modules.
				Val: []string{
					"-f",
					"scan",
				},
				// Only target the 'sanity' list for mainline, as anything more can take a
				// long time.
				ExtraAttr: []string{"group:mainline"},
			},
			{
				Name: "full",
				// List all modules known to be working. Note that we don't run this regularly
				// (reasons noted below), so it's subject to error.
				Val: []string{
					"-f",
					"oce",
					"scan",
					"owe",
					"wpas_wmm_ac",
					"bgscan",
					"kernel",
					"wep",
					"ieee8021x",

					// We can run the dbus module, but it will all be skipped due to
					// missing Python module (pygobject). Include it, in case the library
					// becomes available in the future.
					"dbus",

					"monitor_interface",
					"wpas_config",
					"pmksa_cache",
					"dfs",
					"sae",
					"ap_ft",
					"ssid",
					"cfg80211",
					"radius",
					"eap_proto",
					"connect_cmd",
					"autoscan",

					// Not all suites are enumerated here, but it's useful to list modules
					// which are intentionally *not* run, in case there are subtle reasons
					// why they won't work.
					//
					// Known flaky (offchannel_tx_roc_grpform and
					// offchannel_tx_roc_grpform2).
					//   "offchannel_tx",
				},
				// Not currently targeted for mainline, as the tests can take a long time and
				// are probably only most useful as less-frequent, non-Commit-Queue-blocking
				// usage -- for example, for testing wholesale wpa_supplicant upgrades.
				// Consider running this nightly in the future.
				ExtraAttr: []string{"disabled"},
				// Tests can take a while: 13 minutes for the ~20 modules I first benchmarked.
				// Give some headroom beyond that.
				Timeout: 45 * time.Minute,
			},
		},
	})
}

func HostapHwsim(ctx context.Context, s *testing.State) {
	// Hwsim tests will spin up ~8 virtual clients and ~3 APs. We don't
	// want shill to manage any of them.
	const blacklistArgs = "BLACKLISTED_DEVICES=wlan0,wlan1,wlan2,wlan3,wlan4,wlan5,wlan6,wlan7,hwsim0,hwsim1,hwsim2"

	// Arguments passed to the run-all wrapper script. Useful args:
	//   --vm: tell the test wrapper we're launching directly within a VM.
	//     Among other things, this means we take care of our own logs (and
	//     the wrapper doesn't tar them up into /tmp for us).
	//   --trace: collect additional tracing results via trace-cmd (not
	//     included by default; include dev-util/trace-cmd ebuild if
	//     desired).
	//   -f <module1> [<module2> ...]: run tests from these module(s).
	//   <test1> [<test2> ...]: when not using -f, run individual test cases.
	//
	// If no <testX> or <moduleX> args are provided, run all tests.
	//
	// By default, we only select modules/tests are currently known to pass
	// reliably (defaultTestList), but for manual invocation, one can
	// provide a precise test list via the 'network.HostapHwsim.runArgs'
	// var.
	var testArgs = []string{
		"--vm",
	}
	defaultTestList := s.Param().([]string)

	var runArgs []string
	if testList, ok := s.Var("network.HostapHwsim.runArgs"); ok {
		runArgs = append(testArgs, strings.Fields(testList)...)
	} else {
		runArgs = append(testArgs, defaultTestList...)
	}

	// Get the system wpa_supplicant out of the way; hwsim tests spin up
	// several of their own instances.
	s.Log("Preparing wpa_supplicant and shill")
	if err := upstart.StopJob(ctx, "wpasupplicant"); err != nil {
		s.Fatal("Failed to stop wpasupplicant: ", err)
	}
	defer upstart.StartJob(ctx, "wpasupplicant")

	func() {
		// Don't let recover_duts be confused by a re-starting Shill.
		unlock, err := network.LockCheckNetworkHook(ctx)
		if err != nil {
			s.Fatal("Failed to lock check network hook: ", err)
		}
		// Release the lock after restarting Shill, since the rest of
		// this test doesn't disturb connectivity, and it can run
		// pretty long.
		defer unlock()

		// We don't want Shill to try to manage any of the hwsim WiFi client or
		// AP devices, so re-start Shill with an appropriate blacklist.
		if err := upstart.StopJob(ctx, "shill"); err != nil {
			s.Fatal("Failed to stop shill: ", err)
		}
		if err := upstart.StartJob(ctx, "shill", blacklistArgs); err != nil {
			// One last attempt to get Shill back up / re-set blacklist.
			upstart.RestartJob(ctx, "shill")
			s.Fatal("Failed to start shill with new blacklist: ", err)
		}
	}()

	defer func() {
		// Don't let recover_duts be confused by a re-starting Shill.
		unlock, err := network.LockCheckNetworkHook(ctx)
		if err != nil {
			s.Error("Failed to lock check network hook: ", err)
			// Continue anyway, since it's more important to re-set Shill.
		} else {
			defer unlock()
		}

		// Always re-start Shill at exit, to reset the device blacklist.
		upstart.RestartJob(ctx, "shill")
	}()

	s.Log("Running hwsim tests, args: ", runArgs)
	// Hwsim tests like to run from their own directory.
	if err := os.Chdir("/usr/local/libexec/hostap/tests/hwsim"); err != nil {
		s.Fatal("Failed to chdir: ", err)
	}
	cmd := testexec.CommandContext(ctx, "./run-all.sh", runArgs...)
	// Log to the output directory, so they get captured for later
	// analysis.
	cmd.Env = append(os.Environ(), "LOGDIR="+s.OutDir())

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Fatal("Failed to get stdout: ", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.Fatal("Failed to get stderr: ", err)
	}
	if err := cmd.Start(); err != nil {
		s.Fatal("Failed to start test wrapper: ", err)
	}

	// Log stdout/stderr in real-time. These tests can take a little while,
	// so the progress output is useful.
	multi := io.MultiReader(stdout, stderr)
	in := bufio.NewScanner(multi)
	for in.Scan() {
		s.Log(in.Text())
	}
	if err := in.Err(); err != nil {
		s.Error("Scanner error: ", err)
	}
	if err := cmd.Wait(); err != nil {
		s.Fatal("Hwsim tests failed (see result logs for more info): ", err)
	}
}
