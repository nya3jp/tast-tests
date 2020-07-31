// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
)

const (
	timeout         = 10 * time.Second
	screenRecordDir = "screen-record"
)

// ScreenRecorder is a utility to record the screen during a test scenario.
type ScreenRecorder struct {
	isRecording bool
	videoString string
}

// NewScreenRecorder creates a ScreenRecorder. It only needs to create one
// ScreenRecorder during one test. It chooses the entire desktop as the media
// stream.
func NewScreenRecorder(ctx context.Context, tconn *chrome.TestConn) (*ScreenRecorder, error) {
	expr := `(function() { window.mediaRecorder = null; window.stream = null; window.chunks = [];})()`
	if err := tconn.Eval(ctx, expr, nil); err != nil {
		return nil, errors.Wrap(err, "failed to initialize media recorder")
	}

	// Request to share the screen.
	expr = `(function() {
		window.stream = navigator.mediaDevices.getDisplayMedia({
			audio: false,
			video: {
				cursor: "always"
			}
		})
		})()`
	if err := tconn.Eval(ctx, expr, nil); err != nil {
		return nil, errors.Wrap(err, "failed to request display media")
	}

	// Choose to record the entire desktop/screen with no audio.
	mediaview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Share your screen", ClassName: "DesktopMediaPickerDialogView"}, timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find media picker view")
	}
	defer mediaview.Release(ctx)

	desktopView, err := mediaview.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "DesktopMediaPicker_DesktopMediaSourceView", Role: ui.RoleTypeButton}, timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find the desktop view")
	}
	defer desktopView.Release(ctx)
	if err := desktopView.FocusAndWait(ctx, timeout); err != nil {
		return nil, errors.Wrap(err, "failed to focus on the desktop view")
	}
	if err := desktopView.LeftClick(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click the desktop view")
	}

	shareButton, err := mediaview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Share", Role: ui.RoleTypeButton}, timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find the share button")
	}
	defer shareButton.Release(ctx)
	if err := shareButton.FocusAndWait(ctx, timeout); err != nil {
		return nil, errors.Wrap(err, "failed to focus on the share button")
	}
	if err := shareButton.LeftClick(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click the share button")
	}

	return &ScreenRecorder{isRecording: false, videoString: ""}, nil
}

// Start creates a new media recorder and starts to record the screen. As long as ScreenRecorder
// is not recording, it can start to record again.
func (r *ScreenRecorder) Start(ctx context.Context, tconn *chrome.TestConn) error {
	if r.isRecording == true {
		return errors.New("recorder alreay started")
	}

	expr := `(function() {
		Promise.resolve(window.stream).then(value => {
			window.mediaRecorder = new MediaRecorder(value, {
				videoBitsPerSecond: 100000,
				mimeType: 'video/webm;codecs=h264'
			});
			window.mediaRecorder.ondataavailable = function (e) {
				window.chunks.push(e.data);
			}
			window.mediaRecorder.start();
		})
		})()`
	if err := tconn.Eval(ctx, expr, nil); err != nil {
		return errors.Wrap(err, "failed to start media recorder")
	}
	r.isRecording = true
	return nil
}

// Stop ends the screen recording and stores the encoded base64 string.
func (r *ScreenRecorder) Stop(ctx context.Context, tconn *chrome.TestConn) error {
	if r.isRecording == false {
		return errors.New("recorder hasn't started yet")
	}

	var result string
	expr := `new Promise((resolve, reject) => {
		window.mediaRecorder.onstop = function (e) {
			var blob = new Blob(window.chunks, {'type': 'video/mp4'});
			var reader = new FileReader();
			reader.onload = function () {
				window.chunks = [];
				resolve(reader.result);
			}
			reader.readAsDataURL(blob);
		}
		window.mediaRecorder.stop();
	})`
	if err := tconn.EvalPromise(ctx, expr, &result); err != nil {
		return errors.Wrap(err, "failed to stop media recorder and get video encoded string")
	}
	r.isRecording = false
	r.videoString = result
	return nil
}

// Save saves the latest stored encoded string into a file 'fileName' under
// the directory /tmp/tast/results/latest/tests/[testname]/screen-record
func (r *ScreenRecorder) Save(ctx context.Context, tconn *chrome.TestConn, outDir string, fileName string) error {
	if r.videoString == "" {
		return errors.New("no screen record")
	}
	dir := filepath.Join(outDir, screenRecordDir)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}
	filePath := filepath.Join(dir, fileName)
	if err := ioutil.WriteFile(filePath, []byte(r.videoString), 0644); err != nil {
		return errors.Wrapf(err, "failed to dump video encoded string to %s", filePath)
	}
	return nil
}
