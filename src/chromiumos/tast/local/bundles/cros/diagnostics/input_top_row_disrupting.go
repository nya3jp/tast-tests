// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	da "chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputTopRowDisrupting,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Pressing several disruptive keys won't disrupt the test and affect other keys' states",
		Contacts:     []string{"jeff.lin@cienet.com", "xliu@cienet.com", "cros-peripherals@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "diagnostics",
				Val:  "Diagnostics",
			}, {
				Name: "chrome",
				Val:  "Chrome",
			},
		},
	})
}

func InputTopRowDisrupting(ctx context.Context, s *testing.State) {
	app := s.Param().(string)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.Region("us"), chrome.EnableFeatures("DiagnosticsAppNavigation", "EnableInputInDiagnosticsApp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}

	vh, err := audio.NewVolumeHelper(ctx)
	if err != nil {
		s.Fatal("Failed to new a volume helper: ", err)
	}
	vChecker := &volumeChecker{vh: vh}

	// Get initial mute status.
	muted, err := vh.IsMuted(ctx)
	if err != nil {
		s.Fatal("Failed to check muted: ", err)
	}
	// checkMuted returns a function to check if the current mute status is the same as the given mute status.
	checkMuted := func(muted bool) action.Action {
		return func(ctx context.Context) error {
			newMuted, err := vh.IsMuted(ctx)
			if err != nil {
				return err
			}
			if newMuted != muted {
				return errors.Errorf("muted is %b, want %b", newMuted, muted)
			}
			return nil
		}
	}

	pm, err := power.NewPowerManager(ctx)
	if err != nil {
		s.Fatal("Failed to create a PowerManager object: ", err)
	}
	bChecker := &brightnessChecker{pm: pm}

	getWindowState := func(winName string) (winState ash.WindowStateType, err error) {
		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			s.Log("window: ", w.Title)
			return w.Title == winName
		})
		if err != nil {
			return winState, errors.Wrapf(err, "failed to find %s window", winName)
		}
		return w.State, nil
	}
	// checkWindowState returns a functin to compare the current window state with the expected state.
	checkWindowState := func(winName string, winState ash.WindowStateType) action.Action {
		return func(ctx context.Context) error {
			w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
				return w.Title == winName
			})
			if err != nil {
				return err
			}
			if winState != w.State {
				return errors.Errorf("window state has changed from %v to %v", winState, w.State)
			}
			return nil
		}
	}

	ui := uiauto.New(tconn)
	inoccuousKey := "x"
	clickDisruptiveKey := func(topRowKey, keyNodeName string) action.Action {
		actionName := "verify disruptive key " + topRowKey + " don't disrupt the test"
		return uiauto.NamedAction(actionName, uiauto.Combine(actionName,
			ui.WaitUntilExists(da.KeyNodeFinder(keyNodeName, da.KeyNotPressed).First()),
			kb.AccelPressAction(topRowKey),
			ui.WaitUntilExists(da.KeyNodeFinder(keyNodeName, da.KeyPressed).First()),
			ui.WaitUntilExists(da.DxKeyboardTester),
			ui.WaitUntilExists(da.KeyNodeFinder(inoccuousKey, da.KeyTested).First()),
			kb.AccelReleaseAction(topRowKey),
			ui.WaitUntilExists(da.KeyNodeFinder(keyNodeName, da.KeyTested).First()),
		))
	}

	if app == "Diagnostics" {
		dxRootNode, err := da.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch diagnostics app: ", err)
		}
		defer da.Close(cleanupCtx, tconn)
		defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

		/*
			winState, err := getWindowState("Diagnostics")
			if err != nil {
				s.Fatal("Failed to get window state: ", err)
			}
		*/

		inputTab := da.DxInput.Ancestor(dxRootNode)
		if err := uiauto.Combine("verify disruptive keys don't disrupt the test and won't affect other key state",
			ui.LeftClick(inputTab),
			ui.LeftClick(da.DxInternalKeyboardTestButton),
			// Pressing and releasing an inoccuous key and check it's shown as pressed in the diagram.
			kb.AccelAction(inoccuousKey),
			ui.WaitUntilExists(da.KeyNodeFinder(inoccuousKey, da.KeyTested).First()),
			// Clicking disruptive keys than check tester is still visible and the inoccuous key still shown as tested.
			clickDisruptiveKey(topRow.BrowserBack, "Back"),
			clickDisruptiveKey(topRow.ZoomToggle, "Fullscreen"),
			// checkWindowState("Diagnostics", winState), // Ensure window state hasn't changed.
			// bChecker.set(),
			clickDisruptiveKey(topRow.BrightnessDown, "Display brightness down"),
			// bChecker.check("equal"), // Ensure brightness hasn't changed.
			clickDisruptiveKey(topRow.BrightnessUp, "Display brightness up"),
			// bChecker.check("equal"), // Ensure brightness hasn't changed.
			// vChecker.set(),
			clickDisruptiveKey(topRow.VolumeUp, "Volume up"),
			// vChecker.check("equal"), // Ensure volume hasn't changed.
			clickDisruptiveKey(topRow.VolumeDown, "Volume down"),
			// vChecker.check("equal"), // Ensure volume hasn't changed.
			clickDisruptiveKey(topRow.VolumeMute, "Mute"),
			// checkMuted(muted), // Ensure mute state hasn't changed.
		)(ctx); err != nil {
			s.Fatal("Failed to test disruptive keys: ", err)
		}
	}
	if app == "Chrome" {
		// Open the Google page.
		conn, err := cr.NewConn(ctx, "https://google.com")
		if err != nil {
			s.Fatal("Failed to open google web page: ", err)
		}
		defer conn.Close()
		defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

		if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for page load: ", err)
		}

		// Open the Google page.
		if err := conn.Navigate(ctx, "https://wikipedia.com"); err != nil {
			s.Fatal("Failed to navigate to wikipedia web page: ", err)
		}

		winState, err := getWindowState("Chrome - Wikipedia")
		if err != nil {
			s.Fatal("Failed to get window state: ", err)
		}
		// Swap the expected state.
		if winState == ash.WindowStateFullscreen {
			winState = ash.WindowStateNormal
		} else if winState == ash.WindowStateNormal {
			winState = ash.WindowStateFullscreen
		}

		if err := uiauto.Combine("verify top row keys for browser",
			// Clicking disruptive keys than check tester is still visible and the inoccuous key still shown as tested.
			kb.AccelAction(topRow.BrowserBack),
			func(ctx context.Context) error {
				if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
					return errors.Wrap(err, "failed to wait for page load")
				}
				// Ensure browser goes back to Google.
				_, err := getWindowState("Chrome - Google")
				return err
			},
			kb.AccelAction(topRow.ZoomToggle),
			checkWindowState("Chrome - Google", winState), // Ensure window state hasn't changed.
			bChecker.set(),
			kb.AccelAction(topRow.BrightnessDown),
			bChecker.check("down"),      // Ensure brightness has decreased.
			uiauto.Sleep(1*time.Second), // Wait between brightness changes.
			kb.AccelAction(topRow.BrightnessUp),
			bChecker.check("up"), // Ensure brightness has increased.
			vChecker.set(),
			kb.AccelAction(topRow.VolumeUp),
			vChecker.check("up"),        // Ensure volume has increased.
			uiauto.Sleep(1*time.Second), // Wait between volume changes.
			kb.AccelAction(topRow.VolumeDown),
			vChecker.check("down"), // Ensure volume has decreased.
			kb.AccelAction(topRow.VolumeMute),
			checkMuted(!muted), // Ensure mute state has changed.
		)(ctx); err != nil {
			s.Fatal("Failed to test keys for browser: ", err)
		}
	}
}

type volumeChecker struct {
	vh     *audio.Helper
	volume int
}

func (v *volumeChecker) set() action.Action {
	return func(ctx context.Context) error {
		vol, err := v.vh.GetVolume(ctx)
		if err != nil {
			return err
		}
		v.volume = vol
		return nil
	}
}

// check returns a function to compare the current volume value
// with the initial value to see if it changes at the given direction.
// direction should be "equal", "up", or "down".
func (v *volumeChecker) check(direction string) action.Action {
	return func(ctx context.Context) error {
		newVol, err := v.vh.GetVolume(ctx)
		if err != nil {
			return err
		}
		if (direction == "equal" && newVol != v.volume) ||
			(direction == "up" && newVol <= v.volume) ||
			(direction == "down" && newVol >= v.volume) {
			return errors.Errorf("volume has changed from %f to %f, want %s", v.volume, newVol, direction)
		}
		testing.ContextLogf(ctx, "Volume changed from %d to %d", v.volume, newVol)
		v.volume = newVol
		return nil
	}
}

type brightnessChecker struct {
	brightness float64
	pm         *power.PowerManager
}

func (b *brightnessChecker) set() action.Action {
	return func(ctx context.Context) error {
		newBrightness, err := b.pm.GetScreenBrightnessPercent(ctx)
		if err != nil {
			return err
		}
		b.brightness = newBrightness
		return nil
	}
}

// check returns a function to compare the current screen brightness
// with the initial value to see if it changes at the given direction.
// direction should be "equal", "up", or "down".
func (b *brightnessChecker) check(direction string) action.Action {
	return func(ctx context.Context) error {
		newBrightness, err := b.pm.GetScreenBrightnessPercent(ctx)
		if err != nil {
			return err
		}
		if (direction == "equal" && newBrightness != b.brightness) ||
			(direction == "up" && newBrightness <= b.brightness) ||
			(direction == "down" && newBrightness >= b.brightness) {
			return errors.Errorf("brightness has changed from %f to %f, want %s", b.brightness, newBrightness, direction)
		}
		testing.ContextLogf(ctx, "Brightness changed from %f to %f", b.brightness, newBrightness)
		b.brightness = newBrightness
		return nil
	}
}
