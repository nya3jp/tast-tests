// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package voicerecorder wraps method and constant of voice recorder app for MTBF testing.
package voicerecorder

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

// VoiceRecorder creates a struct that contains *apputil.App
type VoiceRecorder struct {
	app *apputil.App

	latestRecordName     string
	latestRecordDuration time.Duration
}

const (
	pkgName = "com.media.bestrecorder.audiorecorder"

	idPrefix               = pkgName + ":id/"
	startOrStopRecordBtnID = idPrefix + "btn_record_start"
	editFileNameDialogID   = idPrefix + "edt_file_name"
	playCurrentRecordBtnID = idPrefix + "btn_play_current_record"
	fileNameID             = idPrefix + "fragment_detail_name"
	splitRecordBtnID       = idPrefix + "image_split"
	playBtnID              = idPrefix + "play"
	startMarkerID          = idPrefix + "startmarker"
	zoomInID               = idPrefix + "zoom_in"
	tabSettingID           = idPrefix + "tab_setting"
	locPathID              = idPrefix + "location_path"
	locSettingID           = idPrefix + "notification_sdcard"
	internalStorageID      = idPrefix + "node_value"
	locSelectID            = idPrefix + "btn_ok_folder"
	locCloseID             = idPrefix + "btn_close"
	dashboardID            = idPrefix + "sc_view"
	adViewID               = idPrefix + "layout_ads"

	defaultUITimeout = 15 * time.Second
)

// New returns an instance of VoiceRecorder.
func New(ctx context.Context, kb *input.KeyboardEventWriter, tconn *chrome.TestConn, a *arc.ARC) (*VoiceRecorder, error) {
	app, err := apputil.NewApp(ctx, kb, tconn, a, "Voice Recorder", pkgName)
	if err != nil {
		return nil, err
	}
	return &VoiceRecorder{app: app}, nil
}

// Close closes voice recorder app.
func (vr *VoiceRecorder) Close(ctx context.Context, cr *chrome.Chrome, hasError func() bool, outDir string) error {
	return vr.app.Close(ctx, cr, hasError, outDir)
}

// AppName returns the name of voice recorder app.
func (vr *VoiceRecorder) AppName() string {
	return vr.app.AppName
}

// Launch installs app first if the app doesn't exist. And then launches app.
func (vr *VoiceRecorder) Launch(ctx context.Context) error {
	// vr.app.Install() will install the app only if the app doesn't exist.
	if err := vr.app.Install(ctx); err != nil {
		return err
	}

	if err := vr.grantPermissions(ctx); err != nil {
		return errors.Wrap(err, "failed to grant permission")
	}

	if _, err := vr.app.Launch(ctx); err != nil {
		return err
	}

	startOrStopRecordBtn := vr.app.Device.Object(ui.ID(startOrStopRecordBtnID))
	if err := apputil.WaitForExists(startOrStopRecordBtn, defaultUITimeout)(ctx); err != nil {
		return errors.Wrap(err, "voice recorder is not ready")
	}

	return nil
}

// grantPermissions grants permissions to the app.
func (vr *VoiceRecorder) grantPermissions(ctx context.Context) error {
	for _, permission := range []string{
		"android.permission.RECORD_AUDIO",
		"android.permission.READ_EXTERNAL_STORAGE",
	} {
		if err := vr.app.ARC.Command(ctx, "pm", "grant", pkgName, permission).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to grant access permission")
		}
	}

	return nil
}

// UpdateOutDir updates the output directory of recording from default to Downloads/Recorders.
func (vr *VoiceRecorder) UpdateOutDir(ctx context.Context) error {
	// Go to the setting page to check where the output directory of recording is set now.
	if err := apputil.FindAndClick(vr.app.Device.Object(ui.ID(tabSettingID)), defaultUITimeout)(ctx); err != nil {
		return errors.Wrap(err, "failed to click setting tab")
	}

	dashboard := vr.app.Device.Object(ui.ID(dashboardID))
	if err := dashboard.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to wait setting dashboard to exist")
	}

	// To avoid the ad blocking the location path, scroll down to make the location path show.
	// Refer to: https://developer.android.com/reference/androidx/test/uiautomator/UiScrollable#scrollforward_1
	// Perfoem a forward scroll with the default number of scroll steps, 55.
	if _, err := dashboard.ScrollForward(ctx, 55); err != nil {
		return errors.Wrap(err, "failed to scroll forward")
	}

	locPath := vr.app.Device.Object(ui.ID(locPathID))
	if err := locPath.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to find the location path node")
	}

	pathText, err := locPath.GetText(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the location path")
	}

	// If path is not set to Download folder, make the change.
	if !strings.Contains(pathText, "Download/Recorders") {
		if err := uiauto.Combine("update the location path",
			apputil.FindAndClick(vr.app.Device.Object(ui.ID(locSettingID)), defaultUITimeout),
			apputil.FindAndClick(vr.app.Device.Object(ui.ID(internalStorageID)), defaultUITimeout),
			apputil.FindAndClick(vr.app.Device.Object(ui.Text("Download")), defaultUITimeout),
			apputil.FindAndClick(vr.app.Device.Object(ui.ID(locSelectID)), defaultUITimeout),
			apputil.FindAndClick(vr.app.Device.Object(ui.ID(locCloseID)), defaultUITimeout),
		)(ctx); err != nil {
			return err
		}
	}

	if err := vr.app.Device.PressKeyCode(ctx, ui.KEYCODE_BACK, 0); err != nil {
		return errors.Wrap(err, "failed to go back to main page")
	}

	return nil
}

// RecordAudioFor clicks on record button to record audio and
// checks if the recorded file is stored in file system after the recording is completed.
func (vr *VoiceRecorder) RecordAudioFor(cr *chrome.Chrome, fileDuration time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		// Button for starting recording and button for stoping recording are identical object.
		// The share the same id. And there is no text or description to identify them.
		startOrStopRecordBtn := vr.app.Device.Object(ui.ID(startOrStopRecordBtnID))
		testing.ContextLog(ctx, "Start to record sound")
		if err := uiauto.Combine("record sound",
			apputil.FindAndClick(startOrStopRecordBtn, defaultUITimeout), // First click is for starting recording sound.
			uiauto.Sleep(fileDuration),                                   // For recording sound, sleep for some time after clicking recording button.
			apputil.FindAndClick(startOrStopRecordBtn, defaultUITimeout), // Second click is for stopping recording sound.
		)(ctx); err != nil {
			return err
		}

		editFileNameDialog := vr.app.Device.Object(ui.ID(editFileNameDialogID))
		if err := editFileNameDialog.WaitForExists(ctx, defaultUITimeout); err != nil {
			return errors.Wrap(err, "failed to find edit file name")
		}
		name, err := editFileNameDialog.GetText(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get the recorded audio file name in app %q", vr.app.AppName)
		}
		name = name + ".mp3"

		testing.ContextLogf(ctx, "Save the file: %s", name)
		okBtn := vr.app.Device.Object(ui.Text("OK"))
		if err := apputil.FindAndClick(okBtn, defaultUITimeout)(ctx); err != nil {
			return errors.Wrap(err, "failed to save the audio file")
		}

		testing.ContextLog(ctx, "Check whether recorded file is in file system")
		downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
		if err != nil {
			return errors.Wrap(err, "failed to retrieve users Downloads path")
		}

		path := filepath.Join(downloadsPath, "Recorders", name)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return errors.Wrap(err, "file is not in file system")
			}
			return err
		}
		vr.latestRecordDuration = fileDuration
		vr.latestRecordName = name

		return nil
	}
}

// PlayLatestRecord plays the latest recorded file and use uidetection to
// detect playing icon because the frame will be dynamic while playing the record.
func (vr *VoiceRecorder) PlayLatestRecord(ud *uidetection.Context, iconSource string) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Enter playing page which details display on")
		// After clicking playCurrentRecordBtn, the audio will be played automatically, the frame will be dynamic.
		// ADB uiautomator won't get the idle state until the audio finished playing.
		playCurrentRecordBtn := vr.app.Device.Object(ui.ID(playCurrentRecordBtnID))
		if err := apputil.FindAndClick(playCurrentRecordBtn, defaultUITimeout)(ctx); err != nil {
			return err
		}

		// Use uidetection to detect playing icon because the frame will be dynamic while playing the record.
		playingIcon := uidetection.CustomIcon(iconSource).WithinA11yNode(nodewith.Name("Voice Recorder").HasClass("RootView").Role(role.Window))
		if err := ud.WithScreenshotStrategy(uidetection.ImmediateScreenshot).WaitUntilExists(playingIcon)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the playing icon")
		}

		// The app won't be idle while an audio is playing and the uiautomator can only work under idle state.
		// Wait until the audio finished playing to stabilize the following operations.
		if err := uiauto.Sleep(vr.latestRecordDuration)(ctx); err != nil {
			return errors.Wrapf(err, "failed to sleep for %v secs", vr.latestRecordDuration)
		}

		fileObj := vr.app.Device.Object(ui.ID(fileNameID))
		if err := fileObj.WaitForExists(ctx, defaultUITimeout); err != nil {
			return errors.Wrap(err, "failed to find the file name")
		}

		fnText, err := fileObj.GetText(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get the file name of the record at the detail fragment")
		}
		if fnText != vr.latestRecordName {
			return errors.New("the audio is not the one just recorded")
		}
		testing.ContextLogf(ctx, "Found recorded file: %s", fnText)

		return nil
	}
}

// DeleteLatestRecord deletes the audio file created by RecordAudioFor method.
// The file deletion functionality provide by Voice Recorder might comes with ads show up,
// to avoid dealing with ads, here delete those files by os.Remove().
func (vr *VoiceRecorder) DeleteLatestRecord(ctx context.Context, cr *chrome.Chrome) error {
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		return errors.Wrap(err, "failed to retrieve users Downloads path")
	}
	return os.Remove(filepath.Join(downloadsPath, "Recorders", vr.latestRecordName))
}
