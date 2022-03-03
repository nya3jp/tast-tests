// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// "Preconditions

// Chromeos device
// USB or 3.5mm audiojack headset with microphone.

// Procedure
// 1) Plug in a external headset connector(USB or TRRS-3.5mm-audiojack).

// 2) Talk through the external mic using google voice search or https://online-voice-recorder.com or webRTC(http://apprtc.appspot.com/?debug=loopback)
// __a) Verify that audio is recognized and input is coming from the external 3.5mm mic, and not from the built-in microphone.( Alternate speaking directly into the external microphone, and into the built-in microphone. )

// 3) Wiggle microphone connector
// __a) No audio disruptions disruptions or unacceptable noise levels are observed

// 4) Switch the INPUT audio channel from UI shelf status menu to Internal Mic and repeat 2)
// __a) Verify that audio is recognized and input is coming from the onboard mic

// 5) Switch back to the External microphone INPUT channel.
// __a) Input channel in the status menu should switch.

// 6) Record sound with the external mic via 'voice recorder' app
// [or online - YouTube (http://youtube.com/my_webcam)]
// __ Verify the input is from the external mic, and the recorded audio quality is acceptable

// 7)  Unplug the external external mic. Repeat 6)
// __a) Verify that active input is on the onboard mic and channel in menu.

// 8) Compare audio recordings from external and onboard mics.
// __a) Confirm no noticeable quality or volume level changes."

package crostini

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	externalOption = "Mic jack"
	internalOption = "Microphone (internal)"
)

const (
	expectedSearchResult = "where is google headquarter"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Microphones1ExternalMic,
		Desc:         "External Mic",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      time.Hour, // need to human be there, so set timeout longer
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		VarDeps: []string{"ui.gaiaPoolDefault", "FixtureWebUrl"},
	})
}

func Microphones1ExternalMic(ctx context.Context, s *testing.State) {
	// Login option
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("hi")

	testing.Sleep(ctx, time.Minute)

	s.Log("end")

	// Setup test connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	// 1) Plug in a external headset connector(USB or TRRS-3.5mm-audiojack).
	if err := microphones1ExternalMicStep1(ctx, s); err != nil {
		s.Fatal("Failed to execute step 1: ", err)
	}
	// 2) Talk through the external mic using google voice search or https://online-voice-recorder.com or webRTC(http://apprtc.appspot.com/?debug=loopback)
	// __a) Verify that audio is recognized and input is coming from the external 3.5mm mic, and not from the built-in microphone.( Alternate speaking directly into the external microphone, and into the built-in microphone. )
	if err := microphones1ExternalMicStep2(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// 3) Wiggle microphone connector
	// __a) No audio disruptions disruptions or unacceptable noise levels are observed

	// 4) Switch the INPUT audio channel from UI shelf status menu to Internal Mic and repeat 2)
	// __a) Verify that audio is recognized and input is coming from the onboard mic
	if err := microphones1ExternalMicStep4(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// 5) Switch back to the External microphone INPUT channel.
	// __a) Input channel in the status menu should switch.
	if err := microphones1ExternalMicStep5(ctx, s, tconn); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	return
	// 6) Record sound with the external mic via 'voice recorder' app
	// [or online - YouTube (http://youtube.com/my_webcam)]
	// __ Verify the input is from the external mic, and the recorded audio quality is acceptable
	// 7)  Unplug the external external mic. Repeat 6)
	// __a) Verify that active input is on the onboard mic and channel in menu.
	// 8) Compare audio recordings from external and onboard mics.
	// __a) Confirm no noticeable quality or volume level changes."
	if err := microphones1ExternalMicStep6To8(ctx, s, cr); err != nil {
		s.Fatal("Failed to execute step 6,7,8: ", err)
	}
}

func microphones1ExternalMicStep1(ctx context.Context, s *testing.State) error {
	s.Log("Step 1 - Plug in a external headset connector")
	return nil
}

func microphones1ExternalMicStep2(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 2 - Talk through the external mic using google voice search")

	// switch to external mic
	if err := selectAudioOption(ctx, s, tconn, externalOption); err != nil {
		return errors.Wrap(err, "failed to switch to external audio channel")
	}

	// google search
	if err := googleVoiceSearch(ctx, s, cr, tconn, expectedSearchResult); err != nil {
		return errors.Wrap(err, "failed to use google voice search")
	}

	return nil
}

func microphones1ExternalMicStep3(ctx context.Context, s *testing.State) error {
	s.Log("Step 3 - Wiggle microphone connector")
	return nil
}

func microphones1ExternalMicStep4(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Switch the INPUT audio channel from UI shelf status menu to Internal Mic and repeat 2")

	// switch to input
	if err := selectAudioOption(ctx, s, tconn, internalOption); err != nil {
		return errors.Wrap(err, "failed to switch to input audio channel")
	}

	// search
	if err := googleVoiceSearch(ctx, s, cr, tconn, expectedSearchResult); err != nil {
		return errors.Wrap(err, "failed to use google voice search")
	}

	return nil
}

func microphones1ExternalMicStep5(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	s.Log("Step 5 - Switch back to the External microphone INPUT channel")

	if err := selectAudioOption(ctx, s, tconn, externalOption); err != nil {
		return errors.Wrap(err, "failed to switch to external audio channel")
	}

	return nil
}

func microphones1ExternalMicStep6To8(ctx context.Context, s *testing.State, cr *chrome.Chrome) error {

	const (
		pkgName = "com.coffeebeanventures.easyvoicerecorder"
		actName = "com.digipom.easyvoicerecorder.ui.activity.EasyVoiceRecorderActivity"
	)

	s.Log("Opting into Play Store")
	maxAttempts := 1

	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)
	defer func() {
		if s.HasError() {
			if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			}
			if err := a.PullFile(ctx, "/sdcard/window_dump.xml", filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Install app.
	s.Log("Installing app")
	if err := playstore.InstallApp(ctx, a, d, pkgName, -1); err != nil {
		return errors.Wrap(err, "failed to install app")
	}

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to close playstore")
	}

	openAppCommand := testexec.CommandContext(ctx, "adb", "shell", "am", "start", "-n", pkgName+"/"+actName)
	if err := openAppCommand.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start companion Android app using adb")
	}

	// Click GOT IT!
	gotItText := "GOT IT!"
	gotItClass := "android.widget.Button"
	gotItButton := d.Object(ui.ClassName(gotItClass), ui.TextMatches(gotItText))
	if err := gotItButton.WaitForExists(ctx, utils.DefaultUITimeout); err != nil {
		return errors.Wrap(err, "gotItButton doesn't exists")
	}
	if err := gotItButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on gotItButton")
	}

	// Click on allow
	allowText := "ALLOW"
	allowClass := "android.widget.Button"
	allowButton := d.Object(ui.ClassName(allowClass), ui.TextMatches(allowText))
	if err := allowButton.WaitForExists(ctx, utils.DefaultUITimeout); err != nil {
		return errors.Wrap(err, "allowButton doesn't exists")
	}
	if err := allowButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on allowButton")
	}
	if err := allowButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on allowButton")
	}

	for i := 0; i < 2; i++ {
		recordPauseResID := "com.coffeebeanventures.easyvoicerecorder:id/record_pause_button"
		recordPauseClass := "android.widget.ImageButton"
		recordPauseButton := d.Object(ui.ClassName(recordPauseClass), ui.ResourceID(recordPauseResID))
		if err := recordPauseButton.WaitForExists(ctx, utils.DefaultUITimeout); err != nil {
			return errors.Wrap(err, "recordPauseButton doesn't exists")
		}
		if err := recordPauseButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click on recordPauseButton")
		}

		doneResID := "com.coffeebeanventures.easyvoicerecorder:id/done_button"
		doneClass := "android.widget.ImageView"
		doneButton := d.Object(ui.ClassName(doneClass), ui.ResourceID(doneResID))
		if err := doneButton.WaitForExists(ctx, utils.DefaultUITimeout); err != nil {
			return errors.Wrap(err, "doneButton doesn't exists")
		}
		if err := doneButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click on doneButton")
		}

		menuResID := "com.coffeebeanventures.easyvoicerecorder:id/finished_recording_overflow_menu_button"
		menuClass := "android.widget.ImageView"
		menuButton := d.Object(ui.ClassName(menuClass), ui.ResourceID(menuResID))
		if err := menuButton.WaitForExists(ctx, utils.DefaultUITimeout); err != nil {
			return errors.Wrap(err, "menuButton doesn't exists")
		}
		if err := menuButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click on menuButton")
		}

		shareText := "Share"
		shareClass := "android.widget.TextView"
		shareButton := d.Object(ui.ClassName(shareClass), ui.Text(shareText))
		if err := shareButton.WaitForExists(ctx, utils.DefaultUITimeout); err != nil {
			return errors.Wrap(err, "shareButton doesn't exists")
		}
		if err := shareButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click on shareButton")
		}

		saveToFilesText := "Save to Files"
		saveToFiles := d.Object(ui.Text(saveToFilesText))
		if err := saveToFiles.WaitForExists(ctx, utils.DefaultUITimeout); err != nil {
			return errors.Wrap(err, "saveToFiles doesn't exists")
		}
		if err := saveToFiles.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click on saveToFiles")
		}

		saveText := "SAVE"
		save := d.Object(ui.Text(saveText))
		if err := save.WaitForExists(ctx, utils.DefaultUITimeout); err != nil {
			return errors.Wrap(err, "save doesn't exists")
		}
		if err := save.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click on save")
		}

	}

	s.Log("Step 6 - Record sound with the external mic via 'voice recorder' app")
	s.Log("Step 7 - Unplug the external external mic. Repeat 6")
	s.Log("Step 8 - Compare audio recordings from external and onboard mics")
	return nil
}

func selectAudioOption(ctx context.Context, s *testing.State, tconn *chrome.TestConn, deviceName string) error {

	audioSettingsBtn := nodewith.Role(role.Button).Name("Audio settings")
	audioDetailedView := nodewith.ClassName("AudioDetailedView")

	// If audio settings view is open, just return.
	ui := uiauto.New(tconn)

	// Expand the Quick Settings if it is collapsed.
	if err := quicksettings.Expand(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to expend quick settings")
	}
	defer quicksettings.Hide(ctx, tconn)

	// It worth noting that LeftClickUntil will check the condition before doing the first
	// left click. This actually gives time for the UI to be stable before clicking.
	if err := ui.WithTimeout(10*time.Second).LeftClickUntil(audioSettingsBtn, ui.Exists(audioDetailedView))(ctx); err != nil {
		return errors.Wrap(err, "failed to click audioSettingsBtn")
	}

	option := nodewith.Role(role.CheckBox).Name(deviceName)

	if err := ui.DoDefault(option)(ctx); err != nil {
		return errors.Wrap(err, "failed to do default")
	}

	// inputLabel := nodewith.Name("Input").ClassName("Label")
	finder := nodewith.Ancestor(audioDetailedView)
	nodesInfo, err := ui.NodesInfo(ctx, finder)
	if err != nil {
		return err
	}
	s.Log(utils.PrettyPrint(nodesInfo))
	var inputDevices []string

	// find internal mic under "input" label
	var isInput bool
	isInput = false
	for _, node := range nodesInfo {
		if strings.ToLower(node.Name) == "input" {
			isInput = true
		}

		if node.Role == role.CheckBox && isInput == true {
			inputDevices = append(inputDevices, node.Name)
		}
	}

	s.Log(utils.PrettyPrint(inputDevices))

	// fileName := "AAA.txt"
	// filePath := filepath.Join(s.OutDir(), fileName)
	// testing.ContextLog(ctx, "Test failed. Dumping the automation node tree into ", fileName)
	// if err := uiauto.LogRootDebugInfo(ctx, tconn, filePath); err != nil {
	// 	testing.ContextLog(ctx, "Failed to dump: ", err)
	// }

	return nil
}

func googleVoiceSearch(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, expected string) error {

	const (
		googleURL         = "https://www.google.com"
		googleVoiceButton = `document.querySelector('div.XDyW0e').click()`
		voiceRecordURL    = "https://online-voice-recorder.com"
		voiceReocrdButton = `document.querySelector('div.btn-record').click()`
	)

	s.Log("Step 2 - Talk through the external mic using google voice search")

	conn, err := cr.NewConn(ctx, googleURL)
	if err != nil {
		s.Fatal("Failed to navigate to test website: ", err)
	}
	defer conn.Close()

	// another option ->   [aria-label="語音搜尋"]
	// press search by voice
	err = conn.Eval(ctx, googleVoiceButton, nil)
	if err != nil {
		s.Fatal("Failed to execute JS expression: ", err)
	}

	allowFinder := nodewith.Name("Allow").Role(role.Button)

	// click allow button
	ui := uiauto.New(tconn)
	if err := ui.WithPollOpts(testing.PollOptions{Interval: 2 * time.Second, Timeout: 30 * time.Second}).LeftClick(allowFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to click allow")
	}

	// speak sth
	testing.Sleep(ctx, 15*time.Second)

	// verify browser page
	var actual string
	if err := conn.Eval(ctx, "document.documentElement.innerText", &actual); err != nil {
		s.Fatal("Getting page content failed: ", err)
	}

	if strings.Contains(strings.ToLower(actual), expected) {
		s.Fatalf("Unexpected page content: got %q; want %q", actual, expected)
	}

	// close chrome
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	return nil
}
