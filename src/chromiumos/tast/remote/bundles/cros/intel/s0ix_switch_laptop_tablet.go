// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/firmware/suspend"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         S0ixSwitchLaptopTablet,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "During S0ix switch between laptop and tablet mode to resume DUT",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      10 * time.Minute,
	})
}

func S0ixSwitchLaptopTablet(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	// Perform a Chrome login.
	s.Log("Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, h.DUT, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	r := h.Reporter
	var cutoffEvent reporters.Event
	oldEvents, err := r.EventlogList(ctx)
	if err != nil {
		s.Fatal("Failed finding last event: ", err)
	}

	if len(oldEvents) > 0 {
		cutoffEvent = oldEvents[len(oldEvents)-1]
	}

	// Create our suspend context.
	suspendCtx, err := suspend.NewContext(ctx, h)
	if err != nil {
		s.Fatalf("Failed to create suspend context: %s", err)
	}
	defer suspendCtx.Close()

	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		if err := h.EnsureDUTBooted(ctx); err != nil {
			s.Fatal("Failed to ensure the DUT is booted: ", err)
		}
		if err := h.Servo.RunECCommand(ctx, "tabletmode reset"); err != nil {
			s.Fatal("Failed to restore DUT to the original laptop mode setting: ", err)
		}
	}(cleanupCtx)

	slpOpSetPre, pkgOpSetPre, err := powercontrol.SlpAndC10PackageValues(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values before suspend-resume: ", err)
	}

	s.Log("Suspending DUT")
	if err := suspendCtx.SuspendDUT(suspend.StateS0ix, suspend.DefaultSuspendArgs()); err != nil {
		s.Fatalf("Failed to suspend DUT: %s", err)
	}

	// Run EC command to put DUT in tablet mode.
	if err := h.Servo.RunECCommand(ctx, "tabletmode on"); err != nil {
		s.Fatal("Failed to set DUT into tablet mode: ", err)
	}

	if err := verifyEvents(ctx, r, cutoffEvent); err != nil {
		s.Fatal("Failed to verify events: ", err)
	}

	slpOpSetPost, pkgOpSetPost, err := powercontrol.SlpAndC10PackageValues(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values after suspend-resume: ", err)
	}

	if err := powercontrol.AssertSLPAndC10(slpOpSetPre, slpOpSetPost, pkgOpSetPre, pkgOpSetPost); err != nil {
		s.Fatal("Failed to verify SLP and C10 state values: ", err)
	}

	s.Log("Suspending DUT")
	if err := suspendCtx.SuspendDUT(suspend.StateS0ix, suspend.DefaultSuspendArgs()); err != nil {
		s.Fatalf("Failed to suspend DUT: %s", err)
	}

	// Run EC command to put DUT in normal mode.
	if err := h.Servo.RunECCommand(ctx, "tabletmode off"); err != nil {
		s.Fatal("Failed to set DUT into tablet mode: ", err)
	}

	if err := verifyEvents(ctx, r, cutoffEvent); err != nil {
		s.Fatal("Failed to verify events: ", err)
	}

	slpOpSetPostL, pkgOpSetPostL, err := powercontrol.SlpAndC10PackageValues(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values after suspend-resume: ", err)
	}

	if err := powercontrol.AssertSLPAndC10(slpOpSetPre, slpOpSetPostL, pkgOpSetPre, pkgOpSetPostL); err != nil {
		s.Fatal("Failed to verify SLP and C10 state values: ", err)
	}

}

// verifyEvents verifies the required event set for S0ix and EC Mode change.
func verifyEvents(ctx context.Context, r *reporters.Reporter, cutoffEvent reporters.Event) error {
	var requiredEventSets = [][]string{{`S0ix Enter`, `S0ix Exit`}, {`EC Event \| Mode change`}}

	events, err := r.EventlogListAfter(ctx, cutoffEvent)
	if err != nil {
		return errors.Wrap(err, "failed gathering events")
	}

	requiredEventsFound := false
	for _, requiredEventSet := range requiredEventSets {
		foundAllRequiredEventsInSet := true
		for _, requiredEvent := range requiredEventSet {
			reRequiredEvent := regexp.MustCompile(requiredEvent)
			if !eventMessagesContainMatch(ctx, events, reRequiredEvent) {
				foundAllRequiredEventsInSet = false
				break
			}
		}
		if foundAllRequiredEventsInSet {
			requiredEventsFound = true
			break
		}
	}

	if !requiredEventsFound {
		return errors.New("failed as required event missing")
	}

	return nil
}

// eventMessagesContainMatch verifies whether mosys event log contains matching eventlog.
func eventMessagesContainMatch(ctx context.Context, events []reporters.Event, re *regexp.Regexp) bool {
	for _, event := range events {
		if re.MatchString(event.Message) {
			return true
		}
	}
	return false
}
