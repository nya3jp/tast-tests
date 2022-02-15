// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package voicerecorder wraps method and constant of voice recorder app for MTBF testing.
package voicerecorder

import (
	"context"
	"os"
	"path/filepath"
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
	return nil // To be implemented.
}

// RecordAudio clicks on record button to record audio and returns the name of recorded file.
func (vr *VoiceRecorder) RecordAudio(ctx context.Context) (string, error) {
	return "", nil // To be implemented.
}

// PlayFile plays a specified file.
func (vr *VoiceRecorder) PlayFile(fileName string) uiauto.Action {
	return func(c context.Context) error { return nil } // To be implemented.
}

// DeleteAudio deletes the audio file created by RecordSound method.
// The file deletion functionality provide by Voice Recorder might comes with ads show up,
// to avoid dealing with ads, here delete those files by os.Remove().
func (vr *VoiceRecorder) DeleteAudio(fileName string) error {
	return os.Remove(filepath.Join(filesapp.DownloadPath, "Recorders", fileName))
}
