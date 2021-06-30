// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility settings
// in ARC accessibility tests.
package a11y

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	// ArcAccessibilityHelperService is the full class name of ArcAccessibilityHelperService.
	ArcAccessibilityHelperService = "org.chromium.arc.accessibilityhelper/org.chromium.arc.accessibilityhelper.ArcAccessibilityHelperService"

	// ApkName is the name of apk which is used in ARC++ accessibility tests.
	ApkName     = "ArcAccessibilityTest.apk"
	packageName = "org.chromium.arc.testapp.accessibilitytest"

	// CheckBox class name.
	CheckBox = "android.widget.CheckBox"
	// EditText class name.
	EditText = "android.widget.EditText"
	// SeekBar class name.
	SeekBar = "android.widget.SeekBar"
	// TextView class name.
	TextView = "android.widget.TextView"
	// ToggleButton class name.
	ToggleButton = "android.widget.ToggleButton"
)

// TestActivity represents an activity that will be used as a test case.
type TestActivity struct {
	Name  string
	Title string
}

// MainActivity is the struct for the main activity used in test cases.
var MainActivity = TestActivity{".MainActivity", "Main Activity"}

// EditTextActivity is the struct for the edit text activity used in test cases.
var EditTextActivity = TestActivity{".EditTextActivity", "Edit Text Activity"}

// IsEnabledAndroid checks if accessibility is enabled in Android.
func IsEnabledAndroid(ctx context.Context, a *arc.ARC) (bool, error) {
	res, err := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled").Output(testexec.DumpLogOnError)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(res)) == "1", nil
}

// EnabledAndroidAccessibilityServices returns enabled AccessibilityService in Android.
func EnabledAndroidAccessibilityServices(ctx context.Context, a *arc.ARC) ([]string, error) {
	res, err := a.Command(ctx, "settings", "--user", "0", "get", "secure", "enabled_accessibility_services").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(res)), ":"), nil
}

// waitForSpokenFeedbackReady enables spoken feedback.
// A connection to the ChromeVox extension background page is returned, and this will be
// closed by the calling function.
func waitForSpokenFeedbackReady(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) (*a11y.ChromeVoxConn, error) {
	// Wait until spoken feedback is enabled in Android side. It takes longer time for Android
	// a11y to be ready, and thus time out here is longer than others.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := IsEnabledAndroid(ctx, a); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check whether accessibility is enabled in Android"))
		} else if !res {
			return errors.New("accessibility not enabled")
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to ensure accessibility is enabled: ")
	}

	cvconn, err := a11y.NewChromeVoxConn(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "creating connection to ChromeVox extension failed: ")
	}

	return cvconn, nil
}

// RunTest installs the ArcAccessibilityTestApplication, launches it, and waits
// for ChromeVox to be ready. It requires an array activities containing the list of activities
// to run the test cases over, and the currently running activity is passed as a string to f().
func RunTest(ctx context.Context, s *testing.State, activities []TestActivity, f func(context.Context, *a11y.ChromeVoxConn, *chrome.TestConn, TestActivity) error) {
	fullCtx := ctx
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()

	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer crastestclient.Unmute(fullCtx)

	d := s.FixtValue().(*arc.PreData)
	a := d.ARC
	cr := d.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := a11y.SetFeatureEnabled(ctx, tconn, a11y.SpokenFeedback, true); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}
	defer func() {
		if err := a11y.SetFeatureEnabled(fullCtx, tconn, a11y.SpokenFeedback, false); err != nil {
			s.Fatal("Failed to disable spoken feedback: ", err)
		}
	}()

	cvconn, err := waitForSpokenFeedbackReady(ctx, cr, a)
	if err != nil {
		s.Fatal(err) // NOLINT: adb/ui returns loggable errors
	}
	defer cvconn.Close()

	s.Log("Installing and starting test app")
	if err := a.Install(ctx, arc.APKPath(ApkName)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	for _, activity := range activities {
		s.Run(ctx, activity.Name, func(ctx context.Context, s *testing.State) {
			// It takes some time for ArcServiceManager to be ready, so make the timeout longer.
			// TODO(b/150734712): Move this out of each subtest once bug has been addressed.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setArcTouchMode)", true); err != nil {
					return err
				}
				return nil
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatal("Timed out waiting for touch mode: ", err)
			}

			act, err := arc.NewActivity(a, packageName, activity.Name)
			if err != nil {
				s.Fatal("Failed to create new activity: ", err)
			}
			defer act.Close()

			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed to start activity: ", err)
			}
			defer act.Stop(ctx, tconn)

			// TODO(b/161864703): Use chrome.Conn instead of TestConn.
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, &chrome.TestConn{Conn: cvconn.Conn}, "ui_tree"+activity.Name+".txt")

			if err := func() error {
				if err = cvconn.WaitForFocusedNode(ctx, tconn, nodewith.Name(activity.Title).Role(role.Application), 10*time.Second); err != nil {
					return errors.Wrap(err, "failed to wait for initial ChromeVox focus")
				}

				return f(ctx, cvconn, tconn, activity)
			}(); err != nil {
				// TODO(crbug.com/1044446): Take faillog on testing.State.Fatal() invocation.
				screenshotFilename := "screenshot-with-chromevox" + activity.Name + ".png"
				path := filepath.Join(s.OutDir(), screenshotFilename)
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Error("Failed to capture screenshot: ", err)
				} else {
					testing.ContextLogf(ctx, "Saved screenshot to %s", screenshotFilename)
				}
				s.Fatal("Failed to run the test: ", err)
			}
		})
	}
}
