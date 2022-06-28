// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabletModeCheck,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies tablet mode functionality with checking input devices behavior",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromeLoggedIn",
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func TabletModeCheck(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	const (
		onboardKeyboardEvtestString = "AT Translated Set 2 keyboard"
		touchpadEvtestString        = "Touchpad"
		enabledStatusString         = "enabled"
		disabledStatusString        = "disabled"
	)

	if err := evtestEvent(ctx, onboardKeyboardEvtestString, enabledStatusString); err != nil {
		s.Fatal("Failed to verify keyboard event in evtest in clamshell mode: ", err)
	}

	if err := evtestEvent(ctx, touchpadEvtestString, enabledStatusString); err != nil {
		s.Fatal("Failed to verify touchpad event in evtest in clamshell mode: ", err)
	}

	testing.ContextLog(ctx, "Put DUT into tablet mode")
	cleanUp, err := ensureTabletModeEnabled(ctx)
	if err != nil {
		s.Fatal("Failed to put DUT in tablet mode: ", err)
	}
	defer cleanUp(cleanupCtx)

	if err := evtestEvent(ctx, onboardKeyboardEvtestString, disabledStatusString); err != nil {
		s.Fatal("Failed to verify keyboard event in evtest in tablet mode: ", err)
	}

	if err := evtestEvent(ctx, touchpadEvtestString, disabledStatusString); err != nil {
		s.Fatal("Failed to verify touchpad event in evtest in tablet mode: ", err)
	}
}

// setTabletModeUsingEC sets tabletModeAngle for provided lidAngle, hysAngle.
func setTabletModeUsingEC(ctx context.Context, lidAngle, hysAngle int) error {
	tabletLidAngle := strconv.Itoa(lidAngle)
	tabletHysAngle := strconv.Itoa(hysAngle)
	if err := testexec.CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", tabletLidAngle, tabletHysAngle).Run(); err != nil {
		return errors.Wrap(err, "failed to execute tablet_mode_angle command")
	}
	return nil
}

// modeValues returns tabletModeAngle values.
func modeValues(ctx context.Context) (int, int, error) {
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := testexec.CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to retrieve tablet_mode_angle settings")
	}
	m := re.FindSubmatch(out)
	if len(m) != 3 {
		return 0, 0, errors.Wrapf(err, "failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}
	initLidAngle, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to convert initLidAngle to integer")
	}
	initHys, err := strconv.Atoi(string(m[2]))
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to convert initHys to integer")
	}
	return initLidAngle, initHys, nil
}

// ensureTabletModeEnabled makes sure that the tablet mode state
// is enabled using EC tool, which takes care of disabling the Keyboard and Trackpad.
// It returns a function which reverts back to the original state.
func ensureTabletModeEnabled(ctx context.Context) (func(ctx context.Context) error, error) {
	// Get the initial tablet_mode_angle settings to restore at the end of test.
	initLidAngle, initHys, err := modeValues(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get initial tablet_mode_angle values")
	}

	// 'ectool motionsense tablet_mode_angle' commmand returns two values,
	// tablet_mode_angle=0 hys=0 for TabletMode.
	tabletModeAngle, tabletHysAngle := 0, 0

	if initLidAngle != tabletModeAngle || initHys != tabletHysAngle {
		if err = setTabletModeUsingEC(ctx, tabletModeAngle, tabletHysAngle); err != nil {
			return nil, errors.Wrap(err, "failed to set DUT to tablet mode")
		}
	}
	// Always revert to the original state; so it can always be back to the original
	// state even when the state changes in another part of the test script.
	return func(ctx context.Context) error {
		return setTabletModeUsingEC(ctx, initLidAngle, initHys)
	}, nil
}

// evtestEvent verifies event is present in evtest and has expectedDetectionStatus.
func evtestEvent(ctx context.Context, eventReString, expectedDetectionStatus string) error {
	out, _ := exec.Command("evtest").CombinedOutput()
	re := regexp.MustCompile(fmt.Sprintf(`(?i)/dev/input/event([0-9]+):.*%s.*`, eventReString))
	result := re.FindStringSubmatch(string(out))
	touchpadEventNum := ""
	if len(result) == 0 {
		return errors.Errorf("failed to find %q match in evtest command output", eventReString)
	}
	touchpadEventNum = result[1]

	wakeSourceFile := fmt.Sprintf("/sys/class/input/event%s/device/device/power/wakeup", touchpadEventNum)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		sourceOut, err := ioutil.ReadFile(wakeSourceFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read %q file", wakeSourceFile)
		}
		got := strings.TrimSpace(string(sourceOut))
		if !strings.Contains(got, expectedDetectionStatus) {
			return errors.Errorf("unexpected detection status: got %q; want %q", got, expectedDetectionStatus)
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return err
	}
	return nil
}
