// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         S0ixStabilityCheckTabletMode,
		Desc:         "Verifies S0ix stability test in tabletmode",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Vars:         []string{"servo"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      10 * time.Minute,
	})
}
func S0ixStabilityCheckTabletMode(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	dut := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	var (
		c10PkgPattern           = regexp.MustCompile(`C10 : ([A-Za-z0-9]+)`)
		prematureWakePattern    = "Premature wakes: 0"
		suspendFailurePattern   = "Suspend failures: 0"
		firmwareLogErrorPattern = "Firmware log errors: 0"
		s0ixErrorPattern        = "s0ix errors: 0"
	)

	const (
		slpS0File       = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
		pkgCstateFile   = "/sys/kernel/debug/pmc_core/package_cstate_show"
		PowerdConfigCmd = "check_powerd_config --suspend_to_idle; echo $?"
		suspendToIdle   = 1
	)

	// Get the initial tablet_mode_angle settings to restore at the end of test.
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
	if err != nil {
		s.Fatal("Failed to retrieve tablet_mode_angle settings: ", err)
	}
	m := re.FindSubmatch(out)
	if len(m) != 3 {
		s.Fatalf("Failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}
	initLidAngle := m[1]
	initHys := m[2]

	// Set tabletModeAngle to 0 to force the DUT into tablet mode.
	testing.ContextLog(ctx, "Put DUT into tablet mode")
	if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", "0", "0").Run(); err != nil {
		s.Fatal("Failed to set DUT into tablet mode: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctxForCleanUp)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer client.CloseChrome(ctxForCleanUp, &empty.Empty{})

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to power normal press: ", err)
			}
			if err := dut.WaitConnect(waitCtx); err != nil {
				if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
					s.Fatal("Failed to power normal press: ", err)
				}
				if err := dut.WaitConnect(waitCtx); err != nil {
					s.Fatal("Failed to wait connect DUT: ", err)
				}
			}
		}

		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initHys)).Run(); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}

		if err := dut.Conn().CommandContext(ctx, "sh", "-c", "umount /var/lib/power_manager && restart powerd").Run(ssh.DumpLogOnError); err != nil {
			s.Log("Failed to restore powerd settings: ", err)
		}

	}(ctxForCleanUp)

	if err := dut.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf(
		"mkdir -p /tmp/power_manager && "+
			"echo %q > /tmp/power_manager/suspend_to_idle && "+
			"mount --bind /tmp/power_manager /var/lib/power_manager && "+
			"restart powerd", suspendToIdle),
	).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to set suspend to idle: ", err)
	}

	configValue, err := dut.Conn().CommandContext(ctx, "bash", "-c", PowerdConfigCmd).Output()
	if err != nil {
		s.Fatalf("Failed to execute %s command: %v", PowerdConfigCmd, err)
	}
	actualValue := strings.TrimSpace(string(configValue))
	expectedValue := "0"
	if actualValue != expectedValue {
		s.Fatalf("Failed to be in S3 state; expected PowerdConfig value %s; got %s", expectedValue, actualValue)
	}

	slpOpSetPre, err := commandsOutput(ctx, dut, slpS0File)
	if err != nil {
		s.Fatal("Failed to get initial slp s0 value: ", err)
	}

	pkgOpSetOutput, err := commandsOutput(ctx, dut, pkgCstateFile)
	if err != nil {
		s.Fatal("Failed to get initial PkgCstate value: ", err)
	}

	matchSetPre := (c10PkgPattern).FindStringSubmatch(pkgOpSetOutput)
	if matchSetPre == nil {
		s.Fatal("Failed to match pre PkgCstate value: ", pkgOpSetOutput)
	}

	pkgOpSetPre := matchSetPre[1]

	// Expected sleep before performing suspend stress test.
	if err := testing.Sleep(ctx, 8*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	stressOut, err := dut.Conn().CommandContext(ctx, "suspend_stress_test", "-c", "10").Output()
	if err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err)
	}
	suspendErrors := []string{prematureWakePattern, suspendFailurePattern, firmwareLogErrorPattern, s0ixErrorPattern}
	for _, errMsg := range suspendErrors {
		if !strings.Contains(string(stressOut), errMsg) {
			s.Fatalf("Failed was expecting %q, but got failures %s", errMsg, string(stressOut))
		}
	}

	slpOpSetPost, err := commandsOutput(ctx, dut, slpS0File)
	if err != nil {
		s.Fatal("Failed to get slp s0 value after DUT suspend-resume: ", err)
	}

	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("Failed SLP counter value must be different than the value %q noted most recently %q", slpOpSetPre, slpOpSetPost)
	}

	if slpOpSetPost == "0" {
		s.Fatal("Failed SLP counter value must be non-zero, noted is: ", slpOpSetPost)
	}

	pkgOpSetPostOutput, err := commandsOutput(ctx, dut, pkgCstateFile)
	if err != nil {
		s.Fatal("Failed to get PkgCstate value after DUT suspend-resume: ", err)
	}

	matchSetPost := (c10PkgPattern).FindStringSubmatch(pkgOpSetPostOutput)
	if matchSetPost == nil {
		s.Fatal("Failed to match post PkgCstate value: ", pkgOpSetPostOutput)
	}

	pkgOpSetPost := matchSetPost[1]
	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf("Failed Package C10 value %q must be different than value noted earlier %q", pkgOpSetPre, pkgOpSetPost)
	}

	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatal("Failed Package C10 should be non-zero")
	}
}

func commandsOutput(ctx context.Context, dut *dut.DUT, cmdFile string) (string, error) {
	out, err := dut.Conn().CommandContext(ctx, "cat", cmdFile).Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to execute 'cat %s' command", cmdFile)
	}
	return string(out), nil
}
