// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/mlservice"
	"chromiumos/tast/local/power/charge"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AdaptiveCharging,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test that Adaptive Charging functionality works correctly",
		Contacts: []string{
			"dbasehore@google.com",               // test author
			"chromeos-platform-power@google.com", // CrOS platform power developers
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(
			hwdep.Battery(),  // Test doesn't run on ChromeOS devices without a battery.
			hwdep.ChromeEC(), // Test requires Chrome EC to set battery sustainer.
			hwdep.ECFeatureChargeControlV2(),
		),
		Vars: []string{"servo"},
		Params: []testing.Param{{
			Name: "charge_now",
			Val:  testChargeNow,
		}, {
			Name: "settings",
			Val:  testSettings,
		}},
		Timeout: time.Hour, // We only need up to an hour if the battery is low. Otherwise, the test should finish in about 10 minutes.
	})
}

type adaptiveChargingTestFunc = func(ctx context.Context, s *testing.State, cr *chrome.Chrome)

func AdaptiveCharging(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testFunc := s.Param().(adaptiveChargingTestFunc)

	// Stop powerd while we're changing directories that it touches.
	if err := upstart.StopJob(ctx, "powerd"); err != nil {
		s.Fatal("Failed to stop powerd: ", err)
	}
	defer upstart.RestartJob(ctx, "powerd")

	// Create fake charge history to make sure the Adaptive Charging heuristic
	// doesn't disable the feature.
	timeFullDir := "/var/lib/power_manager/charge_history/time_full_on_ac/"
	timeAcDir := "/var/lib/power_manager/charge_history/time_on_ac/"
	storedTimeFullDir := createFakeChargeHistory(s, timeFullDir)
	defer os.Rename(storedTimeFullDir, timeFullDir)
	defer os.RemoveAll(timeFullDir)
	storedTimeAcDir := createFakeChargeHistory(s, timeAcDir)
	defer os.Rename(storedTimeAcDir, timeAcDir)
	defer os.RemoveAll(timeAcDir)

	if err := upstart.EnsureJobRunning(ctx, "powerd"); err != nil {
		s.Fatal("Failed to restart powerd: ", err)
	}

	srvo, err := servo.NewDirect(ctx, s.RequiredVar("servo"))
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer srvo.Close(cleanupCtx)

	// After enabled the AdaptiveCharging feature flag, the setting to enable
	// the feature should be enabled by default.
	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("AdaptiveCharging"),
		chrome.ARCDisabled())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	f, err := mlservice.StartFakeAdaptiveChargingMLService(ctx)
	if err != nil {
		s.Fatal("Failed to start fake Adaptive Charging ML service: ", err)
	}
	defer f.StopService()

	// Putting battery within testable range where the Adaptive Charging
	// notification will show.
	if err := charge.EnsureBatteryWithinRange(ctx, cr, srvo, 80.0, 95.0); err != nil {
		s.Fatalf("Failed to ensure battery percentage within %d%% to %d%%: %v", 80, 95, err)
	}

	if err := upstart.RestartJob(ctx, "powerd"); err != nil {
		s.Fatal("Failed to restart powerd: ", err)
	}

	testFunc(ctx, s, cr)
}

// testChargeNow will verify that clicking the "Fully Charge Now" button that
// shows up via notification when Adaptive Charging starts to delay charge
// successfully cancels Adaptive Charging.
func testChargeNow(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	// Battery sustained will be enabled before the notification shows up.
	pollUntilBatterySustainingState(ctx, s, true)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Chrome Test API Connection: ", err)
	}

	ui := uiauto.New(tconn)
	chargeNowButton := nodewith.Name("Fully charge now").Role(role.Button)
	if err := ui.WithTimeout(time.Minute).WaitUntilExists(chargeNowButton)(ctx); err != nil {
		s.Fatal("Failed to wait for the Charge Now button to exist: ", err)
	}

	// Show quicksettings, which will ensure that the Adaptive Charging
	// notification is visible.
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show quicksettings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := ui.LeftClick(chargeNowButton)(ctx); err != nil {
		s.Fatal("Failed to click Full Charge Now button: ", err)
	}

	// Verify that Adaptive Charging stopped delaying charge.
	pollUntilBatterySustainingState(ctx, s, false)
}

// testSettings will disable the re-enable Adaptive Charging via the Settings
// app.
func testSettings(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Chrome Test API Connection: ", err)
	}

	ui := uiauto.New(tconn)
	toggleAdaptiveCharging := nodewith.Name("Adaptive charging").Role(role.ToggleButton)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "power", ui.WaitUntilExists(toggleAdaptiveCharging))
	if err != nil {
		s.Fatal("Failed to navigate to Setting's Power page: ", err)
	}
	defer settings.Close(ctx)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := ui.LeftClick(toggleAdaptiveCharging)(ctx); err != nil {
		s.Fatal("Failed to toggle Adaptive Charging off: ", err)
	}

	pollUntilBatterySustainingState(ctx, s, false)

	if err := ui.LeftClick(toggleAdaptiveCharging)(ctx); err != nil {
		s.Fatal("Failed to toggle Adaptive Charging on: ", err)
	}

	pollUntilBatterySustainingState(ctx, s, true)
}

// createFakeChargeHistory populates `dir` with fake charge history and stores
// the existing contents in the directory specified by the return value.
func createFakeChargeHistory(s *testing.State, dir string) string {
	storedDir, err := ioutil.TempFile("/tmp", "stored_charge_history.*")
	if err != nil {
		s.Fatal("Failed to copy existing charge history to temporary location: ", err)
	}

	os.Rename(dir, storedDir.Name())

	os.Mkdir(dir, 0666)
	if err != nil {
		s.Fatal("Failed to create directory for temporary charge history: ", err)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	duration := 24 * time.Hour
	contents := []byte("\"" + strconv.FormatInt(duration.Microseconds(), 10) + "\"")
	for i := 0; i < 15; i++ {
		ioutil.WriteFile(filepath.Join(dir, strconv.FormatInt(10*today.Add(time.Duration(-i)*duration).UnixMicro(), 10)), contents, 0666)
	}

	return storedDir.Name()
}

func isBatterySustaining(ctx context.Context, s *testing.State) bool {
	out, err := testexec.CommandContext(ctx, "ectool", "chargecontrol").Output()
	if err != nil {
		s.Fatal("Failed to check battery sustainer: ", err)
	}

	s.Logf("chargecontrol status: %s", out)
	sustainDetect := regexp.MustCompile(`Battery sustainer = on`)
	return sustainDetect.MatchString(string(out))
}

func pollUntilBatterySustainingState(ctx context.Context, s *testing.State, sustaining bool) {
	if err := testing.Poll(ctx, func(c context.Context) error {
		if isBatterySustaining(c, s) != sustaining {
			if sustaining {
				return errors.New("Battery sustainer is still off")
			}
			return errors.New("Battery sustainer is still on")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second}); err != nil {
		s.Fatal("Timeout: ", err)
	}
}
