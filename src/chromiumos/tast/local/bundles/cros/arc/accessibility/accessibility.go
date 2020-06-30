// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accessibility provides functions to assist with interacting with accessibility settings
// in ARC accessibility tests.
package accessibility

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// ArcAccessibilityHelperService is the full class name of ArcAccessibilityHelperService.
	ArcAccessibilityHelperService = "org.chromium.arc.accessibilityhelper/org.chromium.arc.accessibilityhelper.ArcAccessibilityHelperService"

	// ApkName is the name of apk which is used in ARC++ accessibility tests.
	ApkName     = "ArcAccessibilityTest.apk"
	packageName = "org.chromium.arc.testapp.accessibilitytest"

	extURL = "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/chromevox/background/background.html"

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

// Feature represents an accessibility feature in Chrome OS.
type Feature string

// List of accessibility features that interacts with ARC.
const (
	SpokenFeedback Feature = "spokenFeedback"
	SwitchAccess           = "switchAccess"
	SelectToSpeak          = "selectToSpeak"
	FocusHighlight         = "focusHighlight"
)

// focusedNode returns the currently focused node of ChromeVox.
// The returned node should be release by the caller.
func focusedNode(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn) (*ui.Node, error) {
	obj := &chrome.JSObject{}
	if err := cvconn.Eval(ctx, "ChromeVoxState.instance.currentRange.start.node", obj); err != nil {
		return nil, err
	}
	return ui.NewNode(ctx, tconn, obj)
}

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

// chromeVoxExtConn returns a connection to the ChromeVox extension's background page.
// If the extension is not ready, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func chromeVoxExtConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	conn, err := c.NewConnForTarget(ctx, chrome.MatchTargetURL(extURL))
	if err != nil {
		return nil, err
	}

	if err := func() error {
		// Ensure that we don't attempt to use the extension before its APIs are
		// available: https://crbug.com/789313.
		if err := conn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
			return errors.Wrap(err, "timed out waiting for ChromeVox connection to be ready")
		}

		// Export necessary modules which is not exported globally.
		if err := conn.Eval(ctx, `(async () => {
		  if (!window.EventStreamLogger) {
		    window.EventStreamLogger = (await import('/chromevox/background/logging/event_stream_logger.js')).EventStreamLogger;
		    window.LogStore = (await import('/chromevox/background/logging/log_store.js')).LogStore;
		    window.ChromeVoxPrefs = (await import('/chromevox/background/prefs.js')).ChromeVoxPrefs;
		  }
		})()`, nil); err != nil {
			return errors.Wrap(err, "failed to export modules from ChromeVox")
		}

		// Make sure ChromeVoxState is exported globally.
		if err := conn.WaitForExpr(ctx, "ChromeVoxState.instance"); err != nil {
			return errors.Wrap(err, "ChromeVoxState is unavailable")
		}

		if err := chrome.AddTastLibrary(ctx, conn); err != nil {
			return errors.Wrap(err, "failed to introduce tast library")
		}

		return nil
	}(); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// SetFeatureEnabled sets the specified accessibility feature enabled/disabled using the provided connection to the extension.
func SetFeatureEnabled(ctx context.Context, tconn *chrome.TestConn, feature Feature, enable bool) error {
	if err := tconn.Call(ctx, nil, `(feature, enable) => {
		  return tast.promisify(tast.bind(chrome.accessibilityFeatures[feature], "set"))({value: enable});
		}`, feature, enable); err != nil {
		return errors.Wrapf(err, "failed to toggle %v to %t", feature, enable)
	}
	return nil
}

// waitForSpokenFeedbackReady enables spoken feedback.
// A connection to the ChromeVox extension background page is returned, and this will be
// closed by the calling function.
func waitForSpokenFeedbackReady(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) (*chrome.Conn, error) {
	// Wait until spoken feedback is enabled in Android side.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := IsEnabledAndroid(ctx, a); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to check whether accessibility is enabled in Android"))
		} else if !res {
			return errors.New("accessibility not enabled")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to ensure accessibility is enabled")
	}

	cvconn, err := chromeVoxExtConn(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "creating connection to ChromeVox extension failed")
	}

	return cvconn, nil
}

// WaitForFocusedNode polls until the properties of the focused node matches the given params.
// timeout specifies the timeout to use when polling.
func WaitForFocusedNode(ctx context.Context, cvconn *chrome.Conn, tconn *chrome.TestConn, params *ui.FindParams, timeout time.Duration) error {
	// Wait for focusClassName to receive focus.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		focused, err := focusedNode(ctx, cvconn, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		defer focused.Release(ctx)

		if match, err := focused.Matches(ctx, *params); err != nil {
			return testing.PollBreak(err)
		} else if !match {
			return errors.Errorf("focused node is incorrect: got %v, want %v", focused, params)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to get current focus")
	}
	return nil
}

// RunTest installs the ArcAccessibilityTestApplication, launches it, and waits
// for ChromeVox to be ready. It requires an array activities containing the list of activities
// to run the test cases over, and the currently running activity is passed as a string to f().
func RunTest(ctx context.Context, s *testing.State, activities []TestActivity, f func(context.Context, *chrome.Conn, *chrome.TestConn, TestActivity) error) {
	fullCtx := ctx
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()

	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer audio.Unmute(fullCtx)

	d := s.PreValue().(arc.PreData)
	a := d.ARC
	cr := d.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := SetFeatureEnabled(ctx, tconn, SpokenFeedback, true); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}
	defer func() {
		if err := SetFeatureEnabled(fullCtx, tconn, SpokenFeedback, false); err != nil {
			s.Fatal("Failed to disable spoken feedback: ", err)
		}
	}()

	cvconn, err := waitForSpokenFeedbackReady(ctx, cr, a)
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer cvconn.Close()

	s.Log("Installing and starting test app")
	if err := a.Install(ctx, arc.APKPath(ApkName)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

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
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			if err := func() error {
				if err = WaitForFocusedNode(ctx, cvconn, tconn, &ui.FindParams{
					ClassName: TextView,
					Name:      activity.Title,
					Role:      ui.RoleTypeStaticText,
				}, 10*time.Second); err != nil {
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
