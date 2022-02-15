// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/apputil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// VoiceRecorder creates a struct that contains *apputil.App
type VoiceRecorder struct {
	app *apputil.App
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
	tabMainID              = idPrefix + "btn_tab_recorder"
	dashboardID            = idPrefix + "sc_view"

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
	if err := vr.app.Install(ctx); err != nil {
		return err
	}

	if err := vr.app.Launch(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Skip prompts")
	if err := vr.skipPrompts(ctx); err != nil {
		return errors.Wrap(err, "failed to skip prompt")
	}

	testing.ContextLog(ctx, "Wait for app ready")
	startOrStopRecordBtn := vr.app.D.Object(ui.ID(startOrStopRecordBtnID))
	if err := startOrStopRecordBtn.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "voice recorder is not ready")
	}

	return nil
}

// skipPrompts verifies if prompt appears or not. If so, accept it and continue.
func (vr *VoiceRecorder) skipPrompts(ctx context.Context) error {
	// There are 3 possible prompts in total, two of them are identical on UI tree.
	prompts := []*ui.Object{
		vr.app.D.Object(ui.Text("WHILE USING THE APP")),
		vr.app.D.Object(ui.Text("ALLOW")),
		vr.app.D.Object(ui.Text("ALLOW")),
	}
	for _, prompt := range prompts {
		if err := apputil.ClickIfExist(prompt, 3*time.Second)(ctx); err != nil {
			return err
		}
	}

	return nil
}

// UpdateOutDir updates the output directory from default to downloads.
func (vr *VoiceRecorder) UpdateOutDir(ctx context.Context) error {
	// Check path first, see if path match Download/Recorders.
	if err := apputil.FindAndClick(vr.app.D.Object(ui.ID(tabSettingID)), defaultUITimeout)(ctx); err != nil {
		return errors.Wrap(err, "failed to click setting tab")
	}

	// To avoid ad blocking download location, scroll down until download location shows.
	dashboard := vr.app.D.Object(ui.ID(dashboardID))
	if err := dashboard.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to go to setting page")
	}

	locPath := vr.app.D.Object(ui.ID(locPathID))
	if err := dashboard.ScrollTo(ctx, locPath); err != nil {
		return errors.Wrap(err, "failed to scroll to location node")
	}
	if err := locPath.WaitForExists(ctx, defaultUITimeout); err != nil {
		return errors.Wrap(err, "failed to find the location node")
	}

	pathText, err := locPath.GetText(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the location path")
	}

	// If path is not set to Download folder, make the change.
	if !strings.Contains(pathText, "Download/Recorders") {
		testing.ContextLog(ctx, "Setting the output path")

		// Open setting and set location to Download.
		if err := uiauto.Combine("enter setting page and open location of recording",
			apputil.FindAndClick(vr.app.D.Object(ui.ID(locSettingID)), defaultUITimeout),
			apputil.FindAndClick(vr.app.D.Object(ui.ID(internalStorageID)), defaultUITimeout),
			apputil.FindAndClick(vr.app.D.Object(ui.Text("Download")), defaultUITimeout),
			apputil.FindAndClick(vr.app.D.Object(ui.ID(locSelectID)), defaultUITimeout),
			apputil.FindAndClick(vr.app.D.Object(ui.ID(locCloseID)), defaultUITimeout),
		)(ctx); err != nil {
			return err
		}
	}
	if err := apputil.FindAndClick(vr.app.D.Object(ui.ID(tabMainID)), defaultUITimeout)(ctx); err != nil {
		return errors.Wrap(err, "failed to click main tab")
	}

	return nil
}

// RecordAudio clicks on record button to record audio and returns the name of recorded file.
func (vr *VoiceRecorder) RecordAudio(ctx context.Context) (string, error) {
	chromeui := uiauto.New(vr.app.Tconn)

	// Button for starting recording and button for stoping recording are identical object.
	// The share the same id. And there is no text or description to identify them.
	startOrStopRecordBtn := vr.app.D.Object(ui.ID(startOrStopRecordBtnID))
	testing.ContextLog(ctx, "Start to record sound")
	if err := uiauto.Combine("record sound",
		apputil.FindAndClick(startOrStopRecordBtn, defaultUITimeout), // First click is for starting recording sound.
		chromeui.Sleep(10*time.Second),                               // For recording sound, sleep for some time after clicking recording button.
		apputil.FindAndClick(startOrStopRecordBtn, defaultUITimeout), // Second click is for stopping recording sound.
	)(ctx); err != nil {
		return "", err
	}

	editFileNameDialog := vr.app.D.Object(ui.ID(editFileNameDialogID))
	if err := editFileNameDialog.WaitForExists(ctx, defaultUITimeout); err != nil {
		return "", errors.Wrap(err, "failed to find edit file name")
	}
	name, err := editFileNameDialog.GetText(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get the recorded audio file name in app %q", vr.app.AppName)
	}
	name = name + ".mp3"

	testing.ContextLogf(ctx, "Save the file: %s", name)
	okBtn := vr.app.D.Object(ui.Text("OK"))
	if err := apputil.FindAndClick(okBtn, defaultUITimeout)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to save the audio file")
	}

	return name, nil
}

// PlayFile plays a specified file.
func (vr *VoiceRecorder) PlayFile(fileName string) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Enter playing page which details display on")
		// After clicking playCurrentRecordBtn, the audio will be played automatically, the frame will be dynamic.
		// ADB uiautomator won't get the idle state until the audio finished playing.
		playCurrentRecordBtn := vr.app.D.Object(ui.ID(playCurrentRecordBtnID))
		if err := apputil.FindAndClick(playCurrentRecordBtn, defaultUITimeout)(ctx); err != nil {
			return err
		}

		fileObj := vr.app.D.Object(ui.ID(fileNameID))
		if err := fileObj.WaitForExists(ctx, defaultUITimeout); err != nil {
			return errors.Wrap(err, "failed to find the file name")
		}

		fnText, err := fileObj.GetText(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get the file name of the record at the detail fragment")
		}
		if fnText != fileName {
			return errors.New("the audio is not the one just recorded")
		}
		testing.ContextLogf(ctx, "Found recorded file: %s", fnText)

		splitRecordBtn := vr.app.D.Object(ui.ID(splitRecordBtnID))
		closeBtn := vr.app.D.Object(ui.Text("Close"))
		playBtn := vr.app.D.Object(ui.ID(playBtnID))
		startMarker := vr.app.D.Object(ui.ID(startMarkerID))
		zoomIn := vr.app.D.Object(ui.ID(zoomInID))

		// To verify if the audio is playing, enter "editing audio" page.
		return uiauto.Combine("enter editing audio page and play",
			apputil.FindAndClick(splitRecordBtn, defaultUITimeout),
			apputil.ClickIfExist(closeBtn, defaultUITimeout),
			apputil.ClickIfExist(zoomIn, defaultUITimeout), // Zoom in to let start marker disappear from screen easier.
			apputil.ClickIfExist(zoomIn, defaultUITimeout),
			apputil.ClickIfExist(zoomIn, defaultUITimeout),
			apputil.FindAndClick(playBtn, defaultUITimeout),
			func(ctx context.Context) error { return startMarker.WaitUntilGone(ctx, defaultUITimeout) }, // Wait for the audio to finish playing. If start marker doesn't disappear, it means the audio doesn't play.
		)(ctx)
	}
}

// DeleteAudio deletes the audio file created by RecordSound method.
// The file deletion functionality provide by Voice Recorder might comes with ads show up,
// to avoid dealing with ads, here delete those files by os.Remove().
func (vr *VoiceRecorder) DeleteAudio(fileName string) error {
	return os.Remove(filepath.Join(filesapp.DownloadPath, "Recorders", fileName))
}
