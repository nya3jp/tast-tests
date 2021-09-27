// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShutdownPwrbuttonStress,
		Desc:         "Stress test shutdown using power button",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com", "cros-fw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Vars:         []string{"servo", "power.boardtypeIter"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      40 * time.Minute,
	})
}

// ShutdownPwrbuttonStress Perform shutdown using power button.
func ShutdownPwrbuttonStress(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	servoHostPort, ok := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoHostPort, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	defaultIter := 2
	newIter, ok := s.Var("power.boardtypeIter")
	if ok {
		defaultIter, _ = strconv.Atoi(newIter)
	}
	chromeLogin := func() {
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)
		client := security.NewBootLockboxServiceClient(cl.Conn)
		if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}
	pwrOnDut := func() {
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			s.Fatal("Failed to power normal press: ", err)
		}
		wtCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := dut.WaitConnect(wtCtx); err != nil {
			s.Log("Unable to wake up DUT. Retrying")
			if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to power normal press: ", err)
			}
			if err := dut.WaitConnect(wtCtx); err != nil {
				s.Fatal("Failed to wait connect DUT: ", err)
			}
		}
	}
	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			pwrOnDut()
		}
	}(ctx)
	chromeLogin()
	for i := 1; i <= defaultIter; i++ {
		s.Logf("Iteration: %d / %d", i, defaultIter)
		if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
			s.Fatal("Failed to power long press: ", err)
		}
		sdCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		if err := dut.WaitUnreachable(sdCtx); err != nil {
			s.Fatal("Failed to shutdown DUT within 8 sec: error: ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			pwrState, err := pxy.Servo().GetECSystemPowerState(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get power state G3 error")
			}
			if pwrState != "G3" {
				return errors.New("System is not in G3")
			}
			return nil
		}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
			s.Fatal("DUT failed to enter G3 state: ", err)
		}
		pwrOnDut()
		chromeLogin()
		const prevSleepStateCmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
		cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		out, err := dut.Conn().CommandContext(cmdCtx, "sh", "-c", prevSleepStateCmd).Output()
		if err != nil {
			s.Fatal("Failed to execute cbmem command: ", err)
		}
		count, err := strconv.Atoi(strings.Split(strings.Replace(string(out), "\n", "", -1), " ")[1])
		if err != nil {
			s.Fatal("Failed string conversion: ", err)
		}
		if count != 5 {
			s.Fatal("Failed as previous sleep state must be 5")
		}
	}
}
