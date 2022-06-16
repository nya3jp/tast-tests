// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SuspendToS0ixStress,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies suspend stress test with S0ix switching",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		VarDeps:      []string{"servo"},
		Params: []testing.Param{{
			Name:    "bronze",
			Val:     500,
			Timeout: 250 * time.Minute,
		}, {
			Name:    "silver",
			Val:     1000,
			Timeout: 500 * time.Minute,
		}, {
			Name:    "gold",
			Val:     2500,
			Timeout: 1250 * time.Minute,
		}}})
}

func SuspendToS0ixStress(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()

	logPath, err := ioutil.TempDir("", "temp")
	if err != nil {
		s.Fatal("Failed to create temp directory: ", err)
	}
	defer os.RemoveAll(logPath)

	servoSpec := s.RequiredVar("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power-on DUT at cleanup: ", err)
			}
		}

		if err := dut.Conn().CommandContext(ctx, "sh", "-c", "umount /var/lib/power_manager && restart powerd").Run(ssh.DumpLogOnError); err != nil {
			s.Log("Failed to restore powerd settings: ", err)
		}
	}(ctxForCleanUp)

	if err := dut.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf(
		"mkdir -p /tmp/power_manager && "+
			"echo 1 > /tmp/power_manager/suspend_to_idle && "+
			"mount --bind /tmp/power_manager /var/lib/power_manager && "+
			"restart powerd"),
	).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to set suspend to idle: ", err)
	}

	powerdConfigCmd := "check_powerd_config --suspend_to_idle; echo $?"
	configValue, err := dut.Conn().CommandContext(ctx, "bash", "-c", powerdConfigCmd).Output(ssh.DumpLogOnError)
	if err != nil {
		s.Fatalf("Failed to execute %q command: %v", powerdConfigCmd, err)
	}
	got := strings.TrimSpace(string(configValue))
	const want = "0"
	if got != want {
		s.Fatalf("Failed to be in S0ix state: got %s, want %s", got, want)
	}

	const (
		prematureWakePattern    = "Premature wakes: 0"
		suspendFailurePattern   = "Suspend failures: 0"
		firmwareLogErrorPattern = "Firmware log errors: 0"
		s0ixErrorPattern        = "s0ix errors: 0"
	)

	suspendErrors := []string{prematureWakePattern, suspendFailurePattern, firmwareLogErrorPattern, s0ixErrorPattern}

	// Checks poll until no premature wake and/or suspend failures occurs with given poll timeout.
	if testing.Poll(ctx, func(ctx context.Context) error {
		stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", "1").Output(ssh.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to execute suspend_stress_test command")
		}
		for _, errMsg := range suspendErrors {
			if !strings.Contains(string(stressOut), errMsg) {
				return errors.Errorf("failed was expecting %q, but got failures %s", errMsg, string(stressOut))
			}
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		s.Fatal("Failed to perform suspend_stress_test: ", err)
	}

	counter := strconv.Itoa(s.Param().(int))
	s.Logf("Execute: suspend_stress_test for %d counters", s.Param().(int))
	stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", counter,
		fmt.Sprintf("--record_dmesg_dir=%s", logPath), "--suspend_min=15", "--suspend_max=20").Output()
	if err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err, string(stressOut))
	}

	for _, want := range suspendErrors {
		if got := string(stressOut); !strings.Contains(got, want) {
			s.Fatalf("Failed: got %s failures; want %q match", got, want)
		}
	}

}
