// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type touchpadTestParams struct {
	tabletMode      bool
	detectionStatus string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         TouchpadFunctionality,
		Desc:         "Verify Touchpad primary button clicks",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:    "clamshell_mode",
			Fixture: "chromeLoggedInForInputs",
			Val: touchpadTestParams{
				tabletMode:      false,
				detectionStatus: "enabled",
			},
		}, {
			Name:    "tablet_mode",
			Fixture: "chromeLoggedInForInputs",
			Val: touchpadTestParams{
				tabletMode:      true,
				detectionStatus: "disabled",
			},
		}},
	})
}

func TouchpadFunctionality(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testOpt := s.Param().(touchpadTestParams)

	// Get the initial tablet_mode_angle settings to restore at the end of test.
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := testexec.CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
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
	if testOpt.tabletMode {
		testing.ContextLog(ctx, "Put DUT into tablet mode")
		if err := testexec.CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", "0", "0").Run(); err != nil {
			s.Fatal("Failed to set DUT into tablet mode: ", err)
		}
	}

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if err := testexec.CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initHys)).Run(); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}
	}(cleanupCtx)

	if err := touchPadDetection(ctx, testOpt.detectionStatus); err != nil {
		s.Fatal("Failed touchpad functionality check: ", err)
	}
}

// touchPadDetection verifies touchpad detection with expectedStatus of detection.
func touchPadDetection(ctx context.Context, expectedStatus string) error {
	out, _ := exec.Command("evtest").CombinedOutput()
	re := regexp.MustCompile(`(?i)/dev/input/event([0-9]+):.*Touchpad.*`)
	result := re.FindStringSubmatch(string(out))
	touchpadEventNum := ""
	if len(result) > 0 {
		touchpadEventNum = result[1]
	} else {
		return errors.New("touchpad not found in evtest command output")
	}
	wakeSourceFile := fmt.Sprintf("/sys/class/input/event%s/device/device/power/wakeup", touchpadEventNum)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		sourceOut, err := testexec.CommandContext(ctx, "cat", wakeSourceFile).Output()
		if err != nil {
			return errors.Wrapf(err, "failed to execute 'cat %s' command", wakeSourceFile)
		}
		actualStatus := strings.TrimSpace(string(sourceOut))
		if !strings.Contains(actualStatus, expectedStatus) {
			return errors.Errorf("touchpad detection expected to be %q but got %q", expectedStatus, actualStatus)
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 10 * time.Second,
	}); err != nil {
		return errors.Wrapf(err, "touchpad detection status is not %q", expectedStatus)
	}
	return nil
}
