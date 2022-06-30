// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         YoutubeStreamHDMIDisplay,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies Youtube stream with 4K display and check display functionalities on HDMI monitor connected on USB type-C port",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"testcert.p12"},
		Vars:         []string{"ui.cSwitchPort", "ui.domainIP"},
		Fixture:      "chromeLoggedIn",
		Timeout:      7 * time.Minute,
	})
}

func YoutubeStreamHDMIDisplay(ctx context.Context, s *testing.State) {
	// Give 5 seconds to cleanup other resources.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard input: ", err)
	}

	initialBrightness, err := systemBrightness(ctx)
	if err != nil {
		s.Fatal("Failed to get initial brightness: ", err)
	}
	defer kb.Accel(cleanupCtx, "enter")

	var videoSource = videocuj.VideoSrc{
		URL:     "https://www.youtube.com/watch?v=LXb3EKWsInQ",
		Title:   "COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
		Quality: "1440p60",
	}

	// cswitch port ID.
	cSwitchON := s.RequiredVar("ui.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("ui.domainIP")

	// Create C-Switch session that performs hot plug-unplug on USB4 device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create session: ", err)
	}

	cSwitchOFF := "0"
	defer func(ctx context.Context) {
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}

		if err := cswitch.CloseSession(ctx, sessionID, domainIP); err != nil {
			s.Log("Failed to close session: ", err)
		}
	}(cleanupCtx)

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	if err := openSettingsApp(ctx, tconn); err != nil {
		s.Fatal("Failed to find the settings app in the available Chrome apps: ", err)
	}

	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Displays").Role(role.Link))
	if err != nil {
		s.Fatal("Failed to launch os-settings Device page: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cui := uiauto.New(tconn)
	displayName, err := externalDisplayName(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get connected external display name: ", err)
	}

	if err := settingsPage(ctx, tconn, cui, role.Tab, displayName); err != nil {
		s.Fatalf("Failed to click on 'External Display %v' option in Display page: %v", displayName, err)
	}

	// Check if the 4k resolution @3840 x 2160 is getting listed in the drop down menu.
	resolutionMenuParams := nodewith.Name("Resolution").Role(role.PopUpButton)
	if err := leftClickUIElement(ctx, cui, resolutionMenuParams); err != nil {
		s.Fatal("Failed to find and click resolution menu: ", err)
	}

	resolution4kParams := nodewith.Name("3840 x 2160").Role(role.ListBoxOption).First()
	if err := leftClickUIElement(ctx, cui, resolution4kParams); err != nil {
		s.Fatal("Failed to find and click resolution '3840 x 2160': ", err)
	}

	if err := settings.Close(ctx); err != nil {
		s.Fatal("Failed to close settings app: ", err)
	}

	if err := typecutils.CheckDisplayInfo(ctx, true, false); err != nil {
		s.Fatal("Failed to check display info : ", err)
	}

	var uiHandler cuj.UIActionHandler
	if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	videoApp := videocuj.NewYtWeb(cr.Browser(), tconn, kb, videoSource, true, cui, uiHandler)
	if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s: %v", videoSource.URL, err)
	}
	defer videoApp.Close(cleanupCtx)

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	deviceName, deviceType, err := setAudioNodeAsHDMI(ctx, cras)
	if err != nil {
		s.Fatal("Failed to set HDMI as active audio node: ", err)
	}

	if err := verifyHdmiInCrasTest(ctx); err != nil {
		s.Fatal("Failed to verify HDMI as output audio node in cras_test_client command: ", err)
	}

	if err := typecutils.VerifyAudioRoute(ctx, deviceName); err != nil {
		s.Fatalf("Failed to verify audio routing through %q: %v", deviceType, err)
	}

	if err := youtubePlayerFunctionalities(ctx, kb, cui, tconn); err != nil {
		s.Fatal("Failed to perform youtube player functionalities: ", err)
	}

	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}

	decreaseBrightness(ctx, s, topRow, kb)

	// Expected one second time sleep after decreasing brightness to zero.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Increasing brightness level with on-board keyboard key press as cleanup.
	defer increaseBrightness(ctx, s, topRow, kb, initialBrightness)
}

// settingsPage performs UI element find and click, after opening settings page.
func settingsPage(ctx context.Context, tconn *chrome.TestConn, cui *uiauto.Context, role role.Role, name string) error {
	confirm := nodewith.Name(name).Role(role)
	if err := cui.WaitForLocation(confirm)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for element")
	}
	if err := cui.LeftClick(confirm)(ctx); err != nil {
		return errors.Wrap(err, "failed to left click element")
	}
	return nil
}

// waitForChangesInBrightness waits for change in brightness value while calling doBrightnessChange function.
// doBrightnessChange does brightness value change with keyboard BrightnessUp/BrightnessDown keypress.
func waitForChangesInBrightness(ctx context.Context, doBrightnessChange func() error) (float64, error) {
	var curBrightness float64
	preBrightness, err := systemBrightness(ctx)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to get system brightness")
	}
	if err := doBrightnessChange(); err != nil {
		return 0.0, errors.Wrap(err, "failed in calling doBrightnessChange function")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		curBrightness, err = systemBrightness(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get brightness after doBrightnessChange function is called"))
		}
		if preBrightness == curBrightness {
			return errors.New("brightness not changed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return 0.0, errors.Wrap(err, "failed to wait for brightness change")
	}
	return curBrightness, nil
}

// systemBrightness gets the current brightness of the system.
func systemBrightness(ctx context.Context) (float64, error) {
	out, err := testexec.CommandContext(ctx, "backlight_tool", "--get_brightness_percent").Output()
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to execute brightness command")
	}
	sysBrightness, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to parse string into float64")
	}
	return sysBrightness, nil
}

// setSystemBrightness sets the brightness of the system.
func setSystemBrightness(ctx context.Context, percent float64) error {
	if err := exec.Command("backlight_tool", fmt.Sprintf("--set_brightness_percent=%f", percent)).Run(); err != nil {
		return errors.Wrapf(err, "failed to set %f%% brightness", percent)
	}
	return nil
}

// decreaseBrightness performs brightness decrease with keyboard keypress.
func decreaseBrightness(ctx context.Context, s *testing.State, topRow *input.TopRowLayout, kb *input.KeyboardEventWriter) {
	for {
		preBrightness, err := systemBrightness(ctx)
		if err != nil {
			s.Fatal("Failed to get brightness: ", err)
		}
		decBrightness, err := waitForChangesInBrightness(ctx, func() error {
			return kb.Accel(ctx, topRow.BrightnessDown)
		})
		if err != nil {
			s.Fatal(`Failed to change brightness after pressing "BrightnessDown": `, err)
		}
		if decBrightness >= preBrightness {
			s.Fatal("Failed to decrease the brightness")
		}
		if decBrightness == 0.0 {
			break
		}
	}
}

// increaseBrightness performs brightness increase with keyboard keypress.
func increaseBrightness(ctx context.Context, s *testing.State, topRow *input.TopRowLayout, kb *input.KeyboardEventWriter, initialBrightness float64) {
	for {
		preBrightness, err := systemBrightness(ctx)
		if err != nil {
			s.Fatal("Failed to get brightness: ", err)
		}
		incBrightness, err := waitForChangesInBrightness(ctx, func() error {
			return kb.Accel(ctx, topRow.BrightnessUp)
		})
		if err != nil {
			s.Fatal(`Failed to change brightness after pressing "BrightnessUp": `, err)
		}
		if incBrightness <= preBrightness {
			s.Fatal("Failed to increase the brightness")
		}
		if incBrightness == initialBrightness {
			break
		}
	}
}

// externalDisplayName will returns connected external display name.
func externalDisplayName(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var displayName string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		displayInfo, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get external display info")
		}
		if len(displayInfo) < 2 {
			return errors.New("failed please connect external 4K monitor to DUT")
		}
		displayName = displayInfo[1].Name
		if displayName == "" {
			return errors.New("external display name is empty")
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		return "", errors.Wrap(err, "failed to get external display info")
	}
	return displayName, nil
}

// openSettingsApp will check for availability of settings app.
func openSettingsApp(ctx context.Context, tconn *chrome.TestConn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		capps, err := ash.ChromeApps(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		for _, app := range capps {
			if app.AppID == apps.Settings.ID {
				return nil
			}
		}
		return errors.New("settings app not yet found in available Chrome apps")
	}, nil)
}

// setAudioNodeAsHDMI performs setting of active audio node to external HDMI.
func setAudioNodeAsHDMI(ctx context.Context, cras *audio.Cras) (string, string, error) {
	expectedAudioOuputNode := "HDMI"
	// Get current audio output device info.
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get the selected audio device")
	}

	if deviceType != expectedAudioOuputNode {
		if err := cras.SetActiveNodeByType(ctx, expectedAudioOuputNode); err != nil {
			return "", "", errors.Wrapf(err, "failed to select active device %s", expectedAudioOuputNode)
		}
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			return "", "", errors.Wrap(err, "failed to get the selected audio device")
		}
		if deviceType != expectedAudioOuputNode {
			return "", "", errors.Errorf("failed to set the audio node type: got %q; want %q", deviceType, expectedAudioOuputNode)
		}
	}
	return deviceName, deviceType, nil
}

// verifyHdmiInCrasTest verifies connected HDMI is detected in cras_test_client.
func verifyHdmiInCrasTest(ctx context.Context) error {
	out, err := testexec.CommandContext(ctx, "cras_test_client").Output()
	if err != nil {
		return errors.Wrap(err, "failed to exceute cras_test_client command")
	}
	re := regexp.MustCompile(`yes.*HDMI.*2\*`)
	if !re.MatchString(string(out)) {
		return errors.New("failed to select HDMI as output audio node")
	}
	return nil
}

// verifyMirrorModeSwitch verifies mirror mode switching.
func verifyMirrorModeSwitch(ctx context.Context, tconn *chrome.TestConn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		info, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get extrenal display info")
		}
		if info[0].MirroringSourceID == "" {
			return errors.New("DUT is not in mirror mode")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond})
}

// leftClickUIElement performs left click of elementFinder.
func leftClickUIElement(ctx context.Context, cui *uiauto.Context, elementFinder *nodewith.Finder) error {
	if err := cui.LeftClick(elementFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to left click element")
	}
	return nil
}

// youtubePlayerFunctionalities performs various youtube player functionalities.
func youtubePlayerFunctionalities(ctx context.Context, kb *input.KeyboardEventWriter, cui *uiauto.Context, tconn *chrome.TestConn) error {
	if err := kb.Accel(ctx, "f"); err != nil {
		return errors.Wrap(err, "failed to press f key")
	}

	fullScreenText := nodewith.Name(`Exit full screen (f)`).Role(role.Button)
	if err := cui.WaitUntilExists(fullScreenText)(ctx); err != nil {
		return errors.Wrap(err, "failed to check the existence of Exit full screen to valiadte the full screen mode")
	}

	if err := kb.Accel(ctx, "alt+-"); err != nil {
		return errors.Wrap(err, "failed to press alt+- key")
	}

	muteButtonText := nodewith.Name(`Mute (m)`).Role(role.Button)
	if err := cui.WaitUntilExists(muteButtonText)(ctx); err == nil {
		return errors.Wrap(err, "failed to check the existence of Mute (m) to valiadte whether screen is minimized or not")
	}

	if err := kb.Accel(ctx, "alt+-"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+fullscreen")
	}

	if err := kb.Accel(ctx, "alt+="); err != nil {
		return errors.Wrap(err, "failed to press ctrl+fullscreen")
	}

	muteButtonText = nodewith.Name(`Mute (m)`).Role(role.Button)
	if err := cui.WaitUntilExists(muteButtonText)(ctx); err != nil {
		return errors.Wrap(err, "failed to check the existence of Mute (m) to valiadte whether screen is minimized or not")
	}

	if err := kb.Accel(ctx, "alt+="); err != nil {
		return errors.Wrap(err, "failed to press ctrl+fullscreen")
	}

	if err := kb.Accel(ctx, "scale"); err != nil {
		return errors.Wrap(err, "failed to press scale button")
	}

	deskButtonText := nodewith.Name(`Desk 1`).Role(role.Button)
	if err := cui.WaitUntilExists(deskButtonText)(ctx); err == nil {
		return errors.Wrap(err, "failed to check the existence of Desk 1 element to validate window mode")
	}

	if err := kb.Accel(ctx, "scale"); err != nil {
		return errors.Wrap(err, "failed to press scale button")
	}

	if err := kb.Accel(ctx, "ctrl+fullscreen"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+fullscreen")
	}

	if err := verifyMirrorModeSwitch(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to verify mirror mode switch")
	}
	return nil
}
