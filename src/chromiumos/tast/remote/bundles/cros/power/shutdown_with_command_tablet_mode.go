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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	cbmemSleepStateValue = 5
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShutdownWithCommandTabletMode,
		Desc:         "Verifies that system comes back after executing shutdown command in tabletmode",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      10 * time.Minute,
	})
}

func ShutdownWithCommandTabletMode(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

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

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powerOnToDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to wake up DUT at cleanup: ", err)
			}
		}
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initHys)).Run(); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}
	}(ctxForCleanUp)

	powerOffCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(powerOffCtx, "shutdown", "-h", "now").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to execute shutdown command: ", err)
	}

	sdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		s.Fatal("Failed to shutdown DUT: ", err)
	}

	if err := validateG3PowerState(ctx, pxy); err != nil {
		s.Fatal("Failed to enter G3 after shutdown: ", err)
	}

	if err := powerOnToDUT(ctx, pxy, dut); err != nil {
		s.Fatal("Failed to wake up DUT: ", err)
	}

	if err := validateCbmemPrevSleepState(ctx, dut, cbmemSleepStateValue); err != nil {
		s.Fatalf("Failed Previous Sleep state is not %v: %v", cbmemSleepStateValue, err)
	}
}

// validateCbmemPrevSleepState sleep state from cbmem command output.
func validateCbmemPrevSleepState(ctx context.Context, dut *dut.DUT, sleepStateValue int) error {
	const (
		// Command to check previous sleep state.
		prevSleepStateCmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
	)
	out, err := dut.Conn().CommandContext(ctx, "sh", "-c", prevSleepStateCmd).Output()
	if err != nil {
		return errors.Wrapf(err, "failed to execute %s command", prevSleepStateCmd)
	}

	actualOut := strings.TrimSpace(string(out))
	expectedOut := fmt.Sprintf("prev_sleep_state %d", sleepStateValue)

	if !strings.Contains(actualOut, expectedOut) {
		return errors.Errorf("expected %q, but got %q", expectedOut, actualOut)
	}
	return nil
}

// validateG3PowerState verify power state G3 after shutdown.
func validateG3PowerState(ctx context.Context, pxy *servo.Proxy) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		pwrState, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get ec power state")
		}
		if pwrState != "G3" {
			return errors.New("DUT not in G3 state")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// powerOnToDUT performs power normal press to wake DUT.
func powerOnToDUT(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT) error {
	waitCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to power button press")
	}
	if err := dut.WaitConnect(waitCtx); err != nil {
		return errors.Wrap(err, "failed to wait connect DUT")
	}
	return nil
}
