// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package touchpad

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

type touchpadTestParams struct {
	tabletMode      bool
	detectionStatus string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         FunctionalityCheck,
		Desc:         "Verify Touchpad primary button clicks",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "clamshell_mode",
			Val: touchpadTestParams{
				tabletMode:      false,
				detectionStatus: "enabled",
			},
		}, {
			Name: "tablet_mode",
			Val: touchpadTestParams{
				tabletMode:      true,
				detectionStatus: "disabled",
			},
		}},
	})
}

func FunctionalityCheck(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testOpt := s.Param().(touchpadTestParams)

	// Set tabletModeAngle to 0 to force the DUT into tablet mode.
	if testOpt.tabletMode {
		testing.ContextLog(ctx, "Put DUT into tablet mode")
		tabletModeAngle, tabletHysAngle := 0, 0
		cleanUp, err := ash.EnsureTabletModeEnabledWithKeyboardDisabled(ctx, tabletModeAngle, tabletHysAngle)
		if err != nil {
			s.Fatal("Failed to put DUT in tablet mode: ", err)
		}
		defer cleanUp(cleanupCtx)
	}

	// Gets touchpad event number from evtest.
	out, _ := exec.Command("evtest").CombinedOutput()
	re := regexp.MustCompile(`(?i)/dev/input/event([0-9]+):.*Touchpad.*`)
	result := re.FindStringSubmatch(string(out))
	touchpadEventNum := ""
	if len(result) > 0 {
		touchpadEventNum = result[1]
	} else {
		s.Fatal("Failed to find touchpad in evtest command output")
	}

	// Verifies touchpad detection with expected detectionStatus.
	wakeSourceFile := fmt.Sprintf("/sys/class/input/event%s/device/device/power/wakeup", touchpadEventNum)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		sourceOut, err := ioutil.ReadFile(wakeSourceFile)
		if err != nil {
			return testing.PollBreak(errors.Wrapf(err, "failed to read %q file", wakeSourceFile))
		}
		actualStatus := strings.TrimSpace(string(sourceOut))
		if !strings.Contains(actualStatus, testOpt.detectionStatus) {
			return errors.Errorf("unexpected detection status, want %q; got %q", testOpt.detectionStatus, actualStatus)
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 10 * time.Second,
	}); err != nil {
		s.Fatal("Failed touchpad detection: ", err)
	}
}
