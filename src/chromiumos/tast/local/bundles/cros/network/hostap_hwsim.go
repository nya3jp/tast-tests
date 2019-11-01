// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bufio"
	"context"
	"io"
	"os"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HostapHwsim,
		Desc: "Run selected hostap tests using a set of simulated WiFi clients/APs",
		Contacts: []string{
			"briannorris@chromium.org",        // Connectivity team
			"chromeos-kernel-wifi@google.com", // Connectivity team
		},
		Attr: []string{"group:mainline", "informational"},
		// TODO(briannorris): add dependency on at least the
		// net-wireless/hostap-test package -- we probably just expect
		// to run this on betty (or amd64-generic) VMs.
		// SoftwareDeps: []string{""},
	})
}

func HostapHwsim(ctx context.Context, s *testing.State) {
	const (
		// Hwsim tests will spin up ~7 virtual clients and ~3 APs. We
		// don't want shill to manage any of them.
		blacklistArgs = "BLACKLISTED_DEVICES=wlan0,wlan1,wlan2,wlan3,wlan4,wlan5,wlan6,hwsim0,hwsim1,hwsim2"
	)

	// Arguments passed to the run-all wrapper script. Useful args:
	//   --vm: tell the test wrapper we're launching directly within a VM.
	//     Among other things, this means we take care of our own logs (and
	//     the wrapper doesn't tar them up into /tmp for us).
	//   -f <module1> [<module2> ...]: run tests from these module(s).
	//   <test1> [<test2> ...]: when not using -f, run individual test cases.
	//
	// If not <testX> or <moduleX> args, run all tests.
	//
	// By default, we only select modules/tests are currently known to pass
	// reliably.
	var testArgs = []string{
		"--vm",
		// "-f", "scan",
		"eap_proto_pwd_invalid_scalar",
	}

	// Get the system wpa_supplicant out of the way; hwsim tests spin up
	// several of their own instances.
	s.Log("Preparing wpa_supplicant and shill")
	if err := upstart.StopJob(ctx, "wpasupplicant"); err != nil {
		s.Fatal("Failed to stop wpasupplicant: ", err)
	}
	defer upstart.StartJob(ctx, "wpasupplicant")

	// We don't want shill to try to manage any of the hwsim WiFi client or
	// AP devices, so re-start shill with an appropriate blacklist.
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		s.Fatal("Failed to stop shill: ", err)
	}
	// Always re-start shill at exit, in good or bad cases -- we want to
	// reset the device blacklist.
	defer upstart.RestartJob(ctx, "shill")
	if err := upstart.StartJob(ctx, "shill", blacklistArgs); err != nil {
		s.Fatal("Failed to start shill with new blacklist: ", err)
	}

	s.Log("Running hwsim tests, args: ", testArgs)
	// Hwsim tests like to run from their own directory.
	if err := os.Chdir("/usr/local/libexec/hostap/tests/hwsim"); err != nil {
		s.Fatal("Failed to chdir: ", err)
	}
	cmd := testexec.CommandContext(ctx, "./run-all.sh", testArgs...)
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
