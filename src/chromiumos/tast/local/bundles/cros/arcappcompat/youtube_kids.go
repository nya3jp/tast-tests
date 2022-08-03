// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arcappcompat will have tast tests for android apps on Chromebooks.
package arcappcompat

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappcompat/pre"
	"chromiumos/tast/local/bundles/cros/arcappcompat/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// clamshellLaunchForYoutubeKids launches YoutubeKids in clamshell mode.
var clamshellLaunchForYoutubeKids = []testutil.TestCase{
	{Name: "Launch app in Clamshell", Fn: launchAppForYoutubeKids, Timeout: testutil.LaunchTestCaseTimeout},
}

// touchviewLaunchForYoutubeKids launches YoutubeKids in tablet mode.
var touchviewLaunchForYoutubeKids = []testutil.TestCase{
	{Name: "Launch app in Touchview", Fn: launchAppForYoutubeKids, Timeout: testutil.LaunchTestCaseTimeout},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         YoutubeKids,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test for YoutubeKids that installs the app also verifies it is logged in and that the main page is open, checks YoutubeKids correctly changes the window state in both clamshell and touchview mode",
		Contacts:     []string{"mthiyagarajan@chromium.org", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:appcompat", "appcompat_top_apps"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForYoutubeKids,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode_default",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForYoutubeKids,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode_default",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForYoutubeKids,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode_default",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForYoutubeKids,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraAttr:         []string{"appcompat_default"},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForYoutubeKids,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForYoutubeKids,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}, {
			Name: "vm_clamshell_mode",
			Val: testutil.TestParams{
				LaunchTests: clamshellLaunchForYoutubeKids,
				CommonTests: testutil.ClamshellCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on tablet only models.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(testutil.TabletOnlyModels...)),
			Pre:               pre.AppCompatBootedUsingTestAccountPool,
		}, {
			Name: "vm_tablet_mode",
			Val: testutil.TestParams{
				LaunchTests: touchviewLaunchForYoutubeKids,
				CommonTests: testutil.TouchviewCommonTests,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			// TODO(b/189704585): Remove hwdep.SkipOnModel once the solution is found.
			// Skip on clamshell only models.
			ExtraHardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.SkipOnModel(testutil.ClamshellOnlyModels...)),
			Pre:               pre.AppCompatBootedInTabletModeUsingTestAccountPool,
		}},
		Timeout: 20 * time.Minute,
		Vars:    []string{"arcappcompat.gaiaPoolDefault"},
	})
}

// YoutubeKids test uses library for opting into the playstore and installing app.
// Checks YoutubeKids correctly changes the window states in both clamshell and touchview mode.
func YoutubeKids(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.google.android.apps.youtube.kids"
		appActivity = ".browse.SplashScreenActivity"
	)

	testSet := s.Param().(testutil.TestParams)
	testutil.RunTestCases(ctx, s, appPkgName, appActivity, testSet)
}

// launchAppForYoutubeKids verifies YoutubeKids is logged in and
// verify YoutubeKids reached main activity page of the app.
func launchAppForYoutubeKids(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	const (
		agreeButtonText          = "I AGREE"
		doneButtonText           = "DONE"
		enterYourYearPageDes     = "Enter 4 digits for the year you were born. This just verifies your age. It isn't stored."
		enterCorrectNumberText   = "To continue, please enter the correct answer"
		frameworkLayoutClassName = "android.widget.FrameLayout"
		iMAParentButtonText      = "I'M A PARENT"
		imageButtonClassName     = "android.widget.ImageButton"
		nextButtonDes            = "next"
		olderAgesDes             = "Older Ages 9–12"
		progressBarClassName     = "android.widget.ProgressBar"
		selectButtonText         = "SELECT"
		skipButtonText           = "SKIP"
		textViewClassName        = "android.widget.TextView"
		userProfileIconID        = "com.google.android.apps.youtube.kids:id/profile_icon"
	)

	// Click on I am parent button.
	iMAParentButton := d.Object(ui.ClassName(textViewClassName), ui.TextMatches("(?i)"+iMAParentButtonText))
	if err := iMAParentButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("iMAParentButton doesn't exist: ", err)
	} else if err := iMAParentButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on iMAParentButton: ", err)
	}

	// Click on next button.
	nextButton := d.Object(ui.ClassName(imageButtonClassName), ui.DescriptionMatches("(?i)"+nextButtonDes))
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("nextButton doesn't exist: ", err)
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	// Check if enter a correct number page exist.
	checkForCorrectNumberPage := d.Object(ui.ClassName(textViewClassName), ui.TextMatches("(?i)"+enterCorrectNumberText))
	if err := checkForCorrectNumberPage.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("checkForCorrectNumberPage does not exist: ", err)
	} else {
		s.Log("checkForCorrectNumberPage does exist and skipped login to the app")
		return
	}
	// Check if enter your year of born page.
	enterYourYearPage := d.Object(ui.ClassName(textViewClassName), ui.DescriptionMatches("(?i)"+enterYourYearPageDes))
	if err := enterYourYearPage.WaitForExists(ctx, testutil.ShortUITimeout); err != nil {
		s.Log("enterYourYearPage doesn't exist and skipped login to the app: ", err)
		return
	}
	// Verify the age of parent.
	enterAge(ctx, s, tconn, a, d, appPkgName, appActivity)

	// Allow playing the demo video and wait until next button is enabled.
	progressBar := d.Object(ui.ClassName(progressBarClassName))
	nextButton = d.Object(ui.ClassName(imageButtonClassName), ui.DescriptionMatches("(?i)"+nextButtonDes), ui.Enabled(false))
	if err := progressBar.WaitForExists(ctx, testutil.DefaultUITimeout); err == nil {
		s.Log("Wait until progress bar is gone")
		if err := nextButton.WaitUntilGone(ctx, testutil.LongUITimeout); err != nil {
			s.Fatal("Next button not enabled yet: ", err)
		}
	}

	// Click on next button.
	nextButton = d.Object(ui.ClassName(imageButtonClassName), ui.DescriptionMatches("(?i)"+nextButtonDes))
	if err := nextButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("nextButton doesn't exist: ", err)
	} else if err := nextButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nextButton: ", err)
	}

	// Click on skip button.
	skipButton := d.Object(ui.ClassName(textViewClassName), ui.TextMatches("(?i)"+skipButtonText))
	if err := skipButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("skipButton doesn't exist: ", err)
	} else if err := skipButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on skipButton: ", err)
	}

	// Select older ages.
	olderAges := d.Object(ui.ClassName(frameworkLayoutClassName), ui.DescriptionMatches("(?i)"+olderAgesDes))
	if err := olderAges.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("olderAges doesn't exist: ", err)
	} else if err := olderAges.Click(ctx); err != nil {
		s.Fatal("Failed to click on olderAges: ", err)
	}

	// Click on select button.
	selectButton := d.Object(ui.ClassName(textViewClassName), ui.TextMatches("(?i)"+selectButtonText))
	if err := selectButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("selectButton doesn't exist: ", err)
	} else if err := selectButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on selectButton: ", err)
	}

	// Click on Done button.
	doneButton := d.Object(ui.ClassName(textViewClassName), ui.TextMatches("(?i)"+doneButtonText))
	if err := doneButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("doneButton doesn't exist: ", err)
	} else if err := doneButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on doneButton: ", err)
	}

	// Click on agree button.
	agreeButton := d.Object(ui.ClassName(textViewClassName), ui.TextMatches("(?i)"+agreeButtonText))
	if err := agreeButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("agreeButton doesn't exist: ", err)
	} else if err := agreeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on agreeButton: ", err)
	}

	// Check if app is logged in with user profile.
	userProfileIcon := d.Object(ui.ID(userProfileIconID))
	if err := userProfileIcon.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		s.Fatal(" userProfileIcon does not exist: ", err)
	}

	testutil.HandleDialogBoxes(ctx, s, d, appPkgName)
	// Check for launch verifier.
	launchVerifier := d.Object(ui.PackageName(appPkgName))
	if err := launchVerifier.WaitForExists(ctx, testutil.LongUITimeout); err != nil {
		testutil.DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
		s.Fatal("launchVerifier doesn't exists: ", err)
	}
}

// enterAge func to verify age of parent.
func enterAge(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		confirmBtnText    = "CONFIRM"
		nineButtonDes     = "9 button"
		oneButtonDes      = "1 button"
		textViewClassName = "android.widget.TextView"
		zeroButtonDes     = "0 button"
	)
	oneButton := d.Object(ui.ClassName(textViewClassName), ui.DescriptionMatches("(?i)"+oneButtonDes))
	if err := oneButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("oneButton doesn't exist: ", err)
	} else if err := oneButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on oneButton: ", err)
	}

	nineButton := d.Object(ui.ClassName(textViewClassName), ui.DescriptionMatches("(?i)"+nineButtonDes))
	if err := nineButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("nineButton doesn't exist: ", err)
	} else if err := nineButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nineButton: ", err)
	}

	if err := nineButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("nineButton doesn't exist: ", err)
	} else if err := nineButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on nineButton: ", err)
	}

	zeroButton := d.Object(ui.ClassName(textViewClassName), ui.DescriptionMatches("(?i)"+zeroButtonDes))
	if err := zeroButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("zeroButton doesn't exist: ", err)
	} else if err := zeroButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on zeroButton: ", err)
	}

	confirmButton := d.Object(ui.ClassName(testutil.AndroidButtonClassName), ui.TextMatches("(?i)"+confirmBtnText))
	if err := confirmButton.WaitForExists(ctx, testutil.DefaultUITimeout); err != nil {
		s.Log("confirmButton doesn't exist: ", err)
	} else if err := confirmButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on confirmButton: ", err)
	}
}
