// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package typecutils contains constants & helper functions used by the tests in the typec directory.
package typecutils

import (
	"context"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

const displayInfoFile = "/sys/kernel/debug/dri/0/i915_display_info"

// FindConnectedDisplay verifies whether display is connected or not.
// displayName holds type of display whether HDMI or DP.
// totalDisplays holds total number of displays connected.
func FindConnectedDisplay(ctx context.Context, totalDisplays int) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		displayNames := []string{"HDMI", "DP"}
		connectors, err := graphics.ModetestConnectors(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get connectors")
		}
		displayCount := 0
		displayName := ""
		for _, displayName = range displayNames {
			for _, connector := range connectors {
				matched, err := regexp.MatchString("^"+displayName, connector.Name)
				if err != nil {
					return errors.Wrap(err, "failed to match connector")
				}
				if matched && connector.Connected {
					displayCount++
					if displayCount == totalDisplays {
						displayName += displayName
						break
					}
				}
			}
		}
		if displayCount != totalDisplays {
			return errors.Errorf("failed to find no of %s displays: want %d; got %d", displayName, totalDisplays, displayCount)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 1 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to find connected dsiplay")
	}
	return nil
}

// CheckDisplayInfo validates the display info. of connected display.
func CheckDisplayInfo(ctx context.Context, typecHdmi bool) error {
	displayInfoPattern, err := regexp.Compile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
	if err != nil {
		return errors.Wrap(err, "failed to compile regexp")
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
		if typecHdmi == true {
			typecHDMIRe := regexp.MustCompile(`.*DP branch device present.*yes\n.*Type.*HDMI`)
			if !typecHDMIRe.MatchString(string(out)) {
				return errors.New("failed to detect external typec HDMI display")
			}
		} else {
			testing.ContextLog(ctx, "No typec HDMI display connected")
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

	if err := ui.LeftClickUntil(displayFinder, ui.WithTimeout(3*time.Second).WaitUntilGone(displayFinder))(ctx); err != nil {
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

// CheckUSBPdMuxinfo verifies whether USB4=1 or not.
func CheckUSBPdMuxinfo(ctx context.Context, deviceStr string) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "ectool", "usbpdmuxinfo").Output()
		if err != nil {
			return errors.Wrap(err, "failed to run usbpdmuxinfo command")
		}
		if !strings.Contains(string(out), deviceStr) {
			return errors.Wrapf(err, "failed to find %s in usbpdmuxinfo", deviceStr)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to check usb4=1")
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
