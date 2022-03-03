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

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// defaultUITimeout
const (
	defaultUITimeout = 20 * time.Second
)

const (
	googleAudioFile      = "apple.mp3"
	expectedSearchResult = "apple"
	goldenAudioFile      = "golden_sample.wav"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Microphone1ExternalMic,
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
		Data:    []string{"apple.mp3", "golden_sample.wav"},
	})
}

func Microphone1ExternalMic(ctx context.Context, s *testing.State) {

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// 1) Plug in a external headset connector(USB or TRRS-3.5mm-audiojack).
	if err := microphone1ExternalMicStep1(ctx, s); err != nil {
		s.Fatal("Failed to execute step 1: ", err)
	}

	audioOptions, err := getAudioOptionFromUI(ctx, s, tconn)
	if err != nil {
		s.Fatal("Failed to get audio option from ui: ", err)
	}

	s.Log("audio options are : ", audioOptions)

	// copy google file to downloads
	googleAudioFileLocation := filepath.Join(filesapp.DownloadPath, googleAudioFile)
	if err := fsutil.CopyFile(s.DataPath(googleAudioFile), googleAudioFileLocation); err != nil {
		s.Fatalf("Failed to copy the test audio file to %s: %s", googleAudioFileLocation, err)
	}
	defer os.Remove(googleAudioFileLocation)

	// copy golden file to downloads
	goldenAudioFileLocation := filepath.Join(filesapp.DownloadPath, goldenAudioFile)
	if err := fsutil.CopyFile(s.DataPath(goldenAudioFile), goldenAudioFileLocation); err != nil {
		s.Fatalf("Failed to copy the golden audio file to %s: %s", goldenAudioFileLocation, err)
	}
	defer os.Remove(goldenAudioFileLocation)

	// select internal speaker
	if err := selectAudioOption(ctx, s, tconn, audioOptions[0]); err != nil {
		s.Fatal("Failed to select internal speaker: ", err)
	}
	// volume up
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	for i := 0; i < 10; i++ {
		if err := kb.Accel(ctx, "F10"); err != nil {
			s.Fatal("Failed to press F10: ", err)
		}
	}

	// 2) Talk through the external mic using google voice search or https://online-voice-recorder.com or webRTC(http://apprtc.appspot.com/?debug=loopback)
	// __a) Verify that audio is recognized and input is coming from the external 3.5mm mic, and not from the built-in microphone.( Alternate speaking directly into the external microphone, and into the built-in microphone. )
	if err := microphone1ExternalMicStep2(ctx, s, cr, tconn, audioOptions); err != nil {
		s.Fatal("Failed to execute step 2: ", err)
	}

	// 3) Wiggle microphone connector
	// __a) No audio disruptions disruptions or unacceptable noise levels are observed

	// 4) Switch the INPUT audio channel from UI shelf status menu to Internal Mic and repeat 2)
	// __a) Verify that audio is recognized and input is coming from the onboard mic
	if err := microphone1ExternalMicStep4(ctx, s, cr, tconn, audioOptions); err != nil {
		s.Fatal("Failed to execute step 4: ", err)
	}

	// 5) Switch back to the External microphone INPUT channel.
	// __a) Input channel in the status menu should switch.
	if err := microphone1ExternalMicStep5(ctx, s, tconn, audioOptions); err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}

	// 6) Record sound with the external mic via 'voice recorder' app
	// [or online - YouTube (http://youtube.com/my_webcam)]
	// __ Verify the input is from the external mic, and the recorded audio quality is acceptable
	// 7)  Unplug the external external mic. Repeat 6)
	// __a) Verify that active input is on the onboard mic and channel in menu.
	// 8) Compare audio recordings from external and onboard mics.
	// __a) Confirm no noticeable quality or volume level changes."
	if err := microphone1ExternalMicStep6To8(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step 6, 7, 8: ", err)
	}
}

func microphone1ExternalMicStep1(ctx context.Context, s *testing.State) error {

	s.Log("Step 1 - Plug in a external headset connector")

	// there is no fixture to control yet

	return nil
}

func microphone1ExternalMicStep2(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, audioOptions []string) error {

	s.Log("Step 2 - Talk through the external mic using google voice search")

	// select external mic
	if err := selectAudioOption(ctx, s, tconn, audioOptions[3]); err != nil {
		return errors.Wrap(err, "failed to select external mic")
	}

	// google voice search
	if err := googleVoiceSearch(ctx, s, cr, tconn, expectedSearchResult, true); err != nil {
		return errors.Wrap(err, "failed to use google voice search")
	}

	return nil
}

func microphone1ExternalMicStep3(ctx context.Context, s *testing.State) error {
	s.Log("Step 3 - Wiggle microphone connector")
	return nil
}

func microphone1ExternalMicStep4(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, audioOptions []string) error {

	s.Log("Step 4 - Switch the INPUT audio channel from UI shelf status menu to Internal Mic and repeat 2")

	// select internal mic
	if err := selectAudioOption(ctx, s, tconn, audioOptions[2]); err != nil {
		return errors.Wrap(err, "failed to select internal mic")
	}

	// google voice search
	if err := googleVoiceSearch(ctx, s, cr, tconn, expectedSearchResult, false); err != nil {
		return errors.Wrap(err, "failed to use google voice search")
	}

	return nil
}

func microphone1ExternalMicStep5(ctx context.Context, s *testing.State, tconn *chrome.TestConn, audioOptions []string) error {

	s.Log("Step 5 - Switch back to the External microphone INPUT channel")

	if err := selectAudioOption(ctx, s, tconn, audioOptions[3]); err != nil {
		return errors.Wrap(err, "failed to switch to external audio channel")
	}

	return nil
}

func microphone1ExternalMicStep6To8(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	const (
		pkgName = "com.coffeebeanventures.easyvoicerecorder"
		actName = "com.digipom.easyvoicerecorder.ui.activity.EasyVoiceRecorderActivity"
	)

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	maxAttempts := 1

	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
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
		s.Fatal("Failed to install app: ", err)
	}

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close playstore: ", err)
	}

	// open file in downloads folder
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(ctx)
	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}
	if err := files.OpenFile(goldenAudioFile)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", goldenAudioFile, err)
	}

	testing.Sleep(ctx, time.Second)

	// openAppCommand := testexec.CommandContext(ctx, "adb", "shell", "am", "start", "-n", pkgName+"/"+actName)
	// if err := openAppCommand.Run(testexec.DumpLogOnError); err != nil {
	// 	s.Fatal("Failed to start companion Android app using adb")
	// }

	act, err := arc.NewActivity(a, pkgName, actName)
	if err != nil {
		return err
	}

	if err := act.Start(ctx, tconn); err != nil {
		return err
	}

	// Click on GOT IT!
	gotItText := "GOT IT!"
	gotItClass := "android.widget.Button"
	gotItButton := d.Object(ui.ClassName(gotItClass), ui.TextMatches(gotItText))
	if err := gotItButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "gotItButton doesn't exists")
	} else if err := gotItButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on gotItButton")
	}

	// Click on allow
	allowText := "ALLOW"
	allowClass := "android.widget.Button"
	allowButton := d.Object(ui.ClassName(allowClass), ui.TextMatches(allowText))
	if err := allowButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "allowButton doesn't exists")
	} else if err := allowButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on allowButton")
	} else if err := allowButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on allowButton")
	}

	s.Log("Step 6 - Record sound with the external mic via 'voice recorder' app")

	// record voice
	if err := letAppRecord(ctx, s, d); err != nil {
		return errors.Wrap(err, "failed to record voice in app")
	}

	s.Log("Step 7 - Unplug the external external mic. Repeat 6")

	// uplug external mic

	// record voice
	if err := letAppRecord(ctx, s, d); err != nil {
		return errors.Wrap(err, "failed to record voice in app")
	}

	s.Log("Step 8 - Compare audio recordings from external and onboard mics")

	// var videofile1, videofile2 string
	// videofile1 = filepath.Join(filesapp.DownloadPath, "My recording 1.m4a")
	// videofile2 = filepath.Join(filesapp.DownloadPath, "My recording 2.m4a")

	return nil
}

// letAppRecord app would record 40s
func letAppRecord(ctx context.Context, s *testing.State, d *ui.Device) error {

	recordPauseResID := "com.coffeebeanventures.easyvoicerecorder:id/record_pause_button"
	recordPauseClass := "android.widget.ImageButton"
	recordPauseButton := d.Object(ui.ClassName(recordPauseClass), ui.ResourceID(recordPauseResID))
	if err := recordPauseButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "recordPauseButton doesn't exists")
	} else if err := recordPauseButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on recordPauseButton")
	}

	doneResID := "com.coffeebeanventures.easyvoicerecorder:id/done_button"
	doneClass := "android.widget.ImageView"
	doneButton := d.Object(ui.ClassName(doneClass), ui.ResourceID(doneResID))
	if err := doneButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "doneButton doesn't exists")
	} else if err := doneButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on doneButton")
	}

	menuResID := "com.coffeebeanventures.easyvoicerecorder:id/finished_recording_overflow_menu_button"
	menuClass := "android.widget.ImageView"
	menuButton := d.Object(ui.ClassName(menuClass), ui.ResourceID(menuResID))
	if err := menuButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "menuButton doesn't exists")
	} else if err := menuButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on menuButton")
	}

	shareText := "Share"
	shareClass := "android.widget.TextView"
	shareButton := d.Object(ui.ClassName(shareClass), ui.Text(shareText))
	if err := shareButton.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "shareButton doesn't exists")
	} else if err := shareButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on shareButton")
	}

	saveToFilesText := "Save to Files"
	saveToFiles := d.Object(ui.Text(saveToFilesText))
	if err := saveToFiles.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "saveToFiles doesn't exists")
	} else if err := saveToFiles.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on saveToFiles")
	}

	saveText := "SAVE"
	save := d.Object(ui.Text(saveText))
	if err := save.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "save doesn't exists")
	} else if err := save.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on save")
	}

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

	option := nodewith.Role(role.CheckBox).Name(deviceName).First()

	if err := ui.DoDefault(option)(ctx); err != nil {
		return errors.Wrap(err, "failed to do default")
	}

	return nil
}

// getAudioOptionFromUI get audio option from UI
// as expect there shall be 4 options
// speaker (internal)
// external output
// microphone (external)
// internal input
func getAudioOptionFromUI(ctx context.Context, s *testing.State, tconn *chrome.TestConn) ([]string, error) {

	audioSettingsBtn := nodewith.Role(role.Button).Name("Audio settings")
	audioDetailedView := nodewith.ClassName("AudioDetailedView")

	// If audio settings view is open, just return.
	ui := uiauto.New(tconn)

	// Expand the Quick Settings if it is collapsed.
	if err := quicksettings.Expand(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to expend quick settings")
	}
	defer quicksettings.Hide(ctx, tconn)

	// It worth noting that LeftClickUntil will check the condition before doing the first
	// left click. This actually gives time for the UI to be stable before clicking.
	if err := ui.WithTimeout(10*time.Second).LeftClickUntil(audioSettingsBtn, ui.Exists(audioDetailedView))(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click audioSettingsBtn")
	}

	// inputLabel := nodewith.Name("Input").ClassName("Label")
	finder := nodewith.Role(role.CheckBox).Ancestor(audioDetailedView)
	nodesInfo, err := ui.NodesInfo(ctx, finder)
	if err != nil {
		return nil, err
	}

	var audioOptions []string

	// find internal mic under "input" label
	for _, node := range nodesInfo {
		if node.Role == role.CheckBox {
			audioOptions = append(audioOptions, node.Name)
		}
	}

	if len(audioOptions) != 4 {
		return nil, errors.Errorf("Not enough audio option got %d, want 4", len(audioOptions))
	}

	return audioOptions, nil
}

func googleVoiceSearch(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, expected string, firstime bool) error {

	const (
		googleURL         = "https://www.google.com"
		googleVoiceButton = `document.querySelector('div.XDyW0e').click()`
	)

	const (
		voiceRecordURL    = "https://online-voice-recorder.com"
		voiceReocrdButton = `document.querySelector('div.btn-record').click()`
	)

	conn, err := cr.NewConn(ctx, googleURL)
	if err != nil {
		return errors.Wrap(err, "failed to navigate to test website")
	}
	defer conn.Close()

	// maximum chrome browser
	ui := uiauto.New(tconn)
	maxBtn := nodewith.Role(role.Button).Name("Maximize")
	if err := ui.WaitUntilExists(maxBtn)(ctx); err == nil {
		if err := ui.LeftClick(maxBtn)(ctx); err != nil {
			return err
		}
	}

	// declare keyboard
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	// press search by voice
	err = conn.Eval(ctx, googleVoiceButton, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression")
	}

	// click allow button without return error
	if firstime {
		allowFinder := nodewith.Name("Allow").Role(role.Button)

		ui := uiauto.New(tconn)
		if err := ui.WithPollOpts(testing.PollOptions{Interval: 2 * time.Second, Timeout: 30 * time.Second}).LeftClick(allowFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to click allow")
		}
	}

	testing.Sleep(ctx, time.Second)

	// open file in downloads folder
	if err := playAudioFile(ctx, s, tconn); err != nil {
		return errors.Wrap(err, "failed to play audio file")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {

		// get page content
		var actual string
		if err := conn.Eval(ctx, "document.documentElement.innerText", &actual); err != nil {
			return errors.Wrap(err, "failed to get page content")
		}

		// verify page content
		if strings.Contains(strings.ToLower(actual), expected) == false {
			return errors.Errorf("unexpected page content: got %q; want %q", actual, expected)
		}

		return nil

	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify page content")
	}

	if err := kb.Accel(ctx, "Ctrl+W"); err != nil {
		return errors.Wrap(err, "failed to close browser")
	}

	return nil
}

// playAudioFile wwcb server let speaker speak sth
func playAudioFile(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {

	// declare keyboard
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	// open file in downloads folder
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(ctx)
	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}
	if err := files.OpenFile(googleAudioFile)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", googleAudioFile, err)
	}

	// Sample time for the audio to play for 2 seconds.
	testing.Sleep(ctx, 2*time.Second)

	// Closing the audio player.
	defer func() {
		if kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Error("Failed to close Audio player: ", err)
		}
	}()

	return nil
}
