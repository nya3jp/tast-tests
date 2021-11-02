// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HDMIAdapterColdbootG3,
		Desc:         "Verifies USB type-c single port adapter functionality with cold boot operation",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Vars: []string{
			"servo",
			"power.chameleon_addr",
			"power.chameleon_display_port",
		},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      15 * time.Minute,
	})
}

func HDMIAdapterColdbootG3(ctx context.Context, s *testing.State) {
	servoSpec, _ := s.Var("servo")
	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	ctxCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := pwrOnDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
	}(ctxCleanup)

	// Login to chrome and check for connected external display.
	loginChrome := func(ctx context.Context) {
		// Perform a Chrome login.
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)
		client := security.NewBootLockboxServiceClient(cl.Conn)
		if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}

		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
		defer cancel()
		chameleonAddr := s.RequiredVar("power.chameleon_addr")
		// Use chameleon board as extended display. Make sure chameleon is connected.
		che, err := chameleon.New(ctx, chameleonAddr)
		if err != nil {
			s.Fatal("Failed to connect to chameleon board: ", err)
		}
		defer che.Close(cleanupCtx)

		portID := 3 // Use default port 3 for display.
		if port, ok := s.Var("power.chameleon_display_port"); ok {
			portID, err = strconv.Atoi(port)
			if err != nil {
				s.Fatalf("Failed to parse chameleon display port %q: %v", port, err)
			}
		}
		dp, err := che.NewPort(ctx, portID)
		if err != nil {
			s.Fatalf("Failed to create chameleon port %d: %v", portID, err)
		}
		if err := dp.Plug(ctx); err != nil {
			s.Fatal("Failed to plug chameleon port: ", err)
		}

		defer dp.Unplug(cleanupCtx)
		// Wait for DUT to detect external display.
		if err := dp.WaitVideoInputStable(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for video input on chameleon board: ", err)
		}

		if err := detectExternalDisplay(ctx, dut, 1); err != nil {
			s.Fatal("Failed connecting external display: ", err)
		}
	}

	loginChrome(ctx)
	iter := 10
	for i := 1; i <= iter; i++ {
		s.Logf("Iteration: %d/%d ", i, iter)
		if err := performsColdboot(ctx, dut); err != nil {
			s.Fatal("Failed to perform coldboot: ", err)
		}
		if err := verifyEcState(ctx, pxy); err != nil {
			s.Fatal("Failed to enter G3 state: ", err)
		}
		if err := pwrOnDUT(ctx, pxy, dut); err != nil {
			s.Fatal("Failed to power on dut after shutdown: ", err)
		}
		// login chrome after waking from coldboot.
		loginChrome(ctx)
		// prev_sleep_state check.
		const prevSleepStateCmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
		out, err := dut.Conn().CommandContext(ctx, "sh", "-c", prevSleepStateCmd).Output()
		if err != nil {
			s.Fatal("Failed to execute cbmem command: ", err)
		}
		count, err := strconv.Atoi(strings.Split(strings.Replace(string(out), "\n", "", -1), " ")[1])
		if err != nil {
			s.Fatal("String conversion failed: ", err)
		}
		if count != 5 {
			s.Fatalf("Failed to check the sleep state, got %q, want 5", count)
		}
	}
}

func detectExternalDisplay(ctx context.Context, dut *dut.DUT, numberOfDisplays int) error {
	DisplayInfoFile := "/sys/kernel/debug/dri/0/i915_display_info"
	var (
		displayInfoRe   = regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
		connectorInfoRe = regexp.MustCompile(`.*: connectors:\n.\s+\[CONNECTOR:\d+:[HDMI]+.*`)
		connectedRe     = regexp.MustCompile(`\[CONNECTOR:\d+:HDMI.*status: connected`)
		modesRe         = regexp.MustCompile(`modes:\n.*"1920x1080":.60`)
	)
	displayInfoPatterns := []*regexp.Regexp{connectorInfoRe, connectedRe, modesRe}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := dut.Conn().CommandContext(ctx, "cat", DisplayInfoFile).Output()
		if err != nil {
			return errors.Wrap(err, "failed to run display info command")
		}
		matchedString := displayInfoRe.FindAllString(string(out), -1)
		if len(matchedString) != numberOfDisplays {
			return errors.New("connected external display info not found")
		}

		for _, pattern := range displayInfoPatterns {
			if !(pattern).MatchString(string(out)) {
				return errors.Errorf("failed %q error message", pattern)
			}
		}

		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "please connect external display as required")
	}
	return nil
}

func performsColdboot(ctx context.Context, dut *dut.DUT) error {
	testing.ContextLog(ctx, "Performing coldboot")
	if err := dut.Conn().CommandContext(ctx, "shutdown", "-h", "now").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return errors.Wrap(err, "failed to execute shutdown command")
	}
	sdCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		return errors.Wrap(err, "failed to wait for unreachable")
	}
	return nil
}

func pwrOnDUT(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT) error {
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to power normal press")
	}
	if err := dut.WaitConnect(waitCtx); err != nil {
		testing.ContextLog(ctx, "Unable to wake up DUT. Retrying")
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to power normal press")
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			return errors.Wrap(err, "failed to wait connect DUT")
		}
	}
	return nil
}

func verifyEcState(ctx context.Context, pxy *servo.Proxy) error {
	ecStateToCheck := "G3"
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		pwrState, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get power state")
		}
		if pwrState != ecStateToCheck {
			return errors.Errorf("system not in %s", ecStateToCheck)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		return err
	}
	return nil
}
