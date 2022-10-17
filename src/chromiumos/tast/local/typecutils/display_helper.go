// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// Package typecutils contains constants & helper functions used by the tests in the typec directory.

package typecutils

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const displayInfoFile = "/sys/kernel/debug/dri/0/i915_display_info"

// FindConnectedDisplay verifies whether display is connected or not.
// totalDisplays holds total number of displays connected.
func FindConnectedDisplay(ctx context.Context, totalDisplays int) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		displayNames := []string{"HDMI", "DP"}
		connectors, err := graphics.ModetestConnectors(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get connectors")
		}
		displayCount := 0
		connectedDisplayName := ""
		for _, displayName := range displayNames {
			for _, connector := range connectors {
				matched := strings.HasPrefix(connector.Name, displayName)
				if matched && connector.Connected {
					displayCount++
					if displayCount == totalDisplays {
						connectedDisplayName = displayName
						break
					}
				}
			}
		}
		if displayCount != totalDisplays {
			return errors.Errorf("unexpected number of %s displays: want %d; got %d", connectedDisplayName, totalDisplays, displayCount)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 1 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to find connected dsiplay")
	}
	return nil
}

// CheckDisplayInfo validates the display info of connected display.
/*
	Example output from /i915_display_info:
	1. One of Pipe B/C/D etc should contain "active=yes" and "hw: active=yes".
		Sample Output:
		[CRTC:91:pipe B]:
		uapi: enable=yes, active=yes, mode="2256x1504": 60 235690 2256 2304 2336 2536 1504 1507 1513 1549 0x48 0x9
		hw: active=yes, adjusted_mode="2256x1504": 60 235690 2256 2304 2336 2536 1504 1507 1513 1549 0x48 0x9
		pipe src size=2256x1504, dither=no, bpp=24
		num_scalers=2, scaler_users=0 scaler_id=-1, scalers[0]: use=no, mode=0, scalers[1]: use=no, mode=0
		[ENCODER:275:DDI A]: connectors:
			[CONNECTOR:276:HDMI-1]
		[PLANE:31:plane 1A]: type=PRI

	2. Connector should contain the information of connector used HDMI/DP.
		Sample output:
		[CONNECTOR:276:HDMI-A-1]: status: connected
		physical dimensions: 280x190mm
		subpixel order: Unknown
		CEA rev: 0
*/
func CheckDisplayInfo(ctx context.Context, typecHdmiConnector, typecDpConnector bool) error {
	displayInfoPattern, err := regexp.Compile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
	if err != nil {
		return errors.Wrap(err, "failed to compile regexp")
	}

	var displayPattern *regexp.Regexp
	if typecDpConnector {
		displayPattern = regexp.MustCompile(`\[CONNECTOR:\d+:DP.*status: connected((.|\n)*)DP branch device present: no`)
	}
	if typecHdmiConnector {
		displayPattern = regexp.MustCompile(`.*DP branch device present.*yes\n.*Type.*HDMI`)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := ioutil.ReadFile(displayInfoFile)
		if err != nil {
			return errors.Wrap(err, "failed to read display info file ")
		}
		found := displayInfoPattern.MatchString(string(out))
		if !found {
			return errors.New("failed to verify external display info in i915_display_info")
		}
		if !displayPattern.MatchString(string(out)) {
			return errors.Errorf("failed %q error message. No typec display found", displayPattern)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to check display info")
	}
	return nil
}

// SetMirrorDisplay sets DUT to mirror mode.
// tconn holds test API connection.
// set holds boolean value true(set mirror display), false(unset mirror display).
func SetMirrorDisplay(ctx context.Context, tconn *chrome.TestConn, set bool) error {
	ui := uiauto.New(tconn)
	displayFinder := nodewith.Name("Displays").Role(role.Link).Ancestor(ossettings.WindowFinder)

	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Device").Role(role.Link))
	if err != nil {
		return errors.Wrap(err, "failed to launch os-settings Device page")
	}
	defer settings.Close(ctx)

	if err := ui.DoDefaultUntil(displayFinder, ui.WithTimeout(3*time.Second).WaitUntilGone(displayFinder))(ctx); err != nil {
		return errors.Wrap(err, "failed to launch display page")
	}

	mirrorFinder := nodewith.Name("Mirror Built-in display").Role(role.CheckBox).Ancestor(ossettings.WindowFinder)
	// Find the node info for the mirror checkbox.
	nodeInfo, err := ui.Info(ctx, mirrorFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the mirror checkbox")
	}

	if set {
		// Set mirror display if its not set already.
		if nodeInfo.Checked == "false" {
			if err := ui.LeftClick(mirrorFinder)(ctx); err != nil {
				return errors.Wrap(err, "failed to click mirror display")
			}
		}
	} else {
		// Unset mirror display if its not unset already.
		if nodeInfo.Checked == "true" {
			if err := ui.LeftClick(mirrorFinder)(ctx); err != nil {
				return errors.Wrap(err, "failed to click mirror display")
			}
		}
	}
	return nil
}

// VerifyDisplay4KResolution verifies whether the connected display has 4k resolution or not.
func VerifyDisplay4KResolution(ctx context.Context) error {
	out, err := ioutil.ReadFile(displayInfoFile)
	if err != nil {
		return errors.Wrap(err, "failed to run display info command ")
	}
	if !(strings.Contains(string(out), "3840x2160") || strings.Contains(string(out), "4096x2160")) {
		return errors.Wrap(err, "failed to find 4K HDMI/DP display")
	}
	return nil
}

// ExtendedDisplayWindowClassName obtains the class name of the root window on the extended display.
func ExtendedDisplayWindowClassName(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	ui := uiauto.New(tconn)

	// Root window on extended display has the class name in RootWindow-<id> format.
	// We found extended display window could be RootWindow-1, or RootWindow-2.
	// Here we try 1 to 10.
	for i := 1; i <= 10; i++ {
		className := fmt.Sprintf("RootWindow-%d", i)
		win := nodewith.ClassName(className).Role(role.Window)
		if err := ui.Exists(win)(ctx); err == nil {
			return className, nil
		}
	}
	return "", errors.New("failed to find any window with class name RootWindow-1 to RootWindow-10")
}

// SwitchWindowToDisplay switches current window to expected display.
func SwitchWindowToDisplay(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, externalDisplay bool) action.Action {
	return func(ctx context.Context) error {

		var expectedRootWindow *nodewith.Finder
		var display string
		ui := uiauto.New(tconn)
		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.IsActive && w.IsFrameVisible
		})
		if err != nil {
			return errors.Wrap(err, "failed to get current active window")
		}
		if externalDisplay {
			display = "external display"
			extendedWinClassName, err := ExtendedDisplayWindowClassName(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to find root window on external display")
			}
			expectedRootWindow = nodewith.ClassName(extendedWinClassName).Role(role.Window)
		} else {
			display = "internal display"
			// Root window on built-in display.
			expectedRootWindow = nodewith.ClassName("RootWindow-0").Role(role.Window)
		}
		currentWindow := nodewith.Name(w.Title).Role(role.Window)
		expectedWindow := currentWindow.Ancestor(expectedRootWindow).First()
		if err := ui.Exists(expectedWindow)(ctx); err != nil {
			testing.ContextLog(ctx, "Expected window not found: ", err)
			testing.ContextLogf(ctx, "Switch window %q to %s", w.Title, display)
			return uiauto.Combine("switch window to "+display,
				kb.AccelAction("Search+Alt+M"),
				ui.WithTimeout(3*time.Second).WaitUntilExists(expectedWindow),
			)(ctx)
		}
		return nil
	}
}

// VerifyAudioRoute checks whether audio is routing via deviceName or not.
func VerifyAudioRoute(ctx context.Context, deviceName string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			return errors.Wrap(err, "failed to detect running output device")
		}
		if deviceName != devName {
			return errors.Wrapf(err, "failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		return errors.Wrapf(err, "timeout waiting for %q", deviceName)
	}
	return nil
}
