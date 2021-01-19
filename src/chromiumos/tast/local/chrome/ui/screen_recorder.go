// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

const (
	timeout = 10 * time.Second
)

// ScreenRecorder is a utility to record the screen during a test scenario.
type ScreenRecorder struct {
	isRecording   bool
	videoRecorder *chrome.JSObject
	result        string
}

// NewScreenRecorder creates a ScreenRecorder. It only needs to create one
// ScreenRecorder during one test. It chooses the entire desktop as the media
// stream.
func NewScreenRecorder(ctx context.Context, tconn *chrome.TestConn) (*ScreenRecorder, error) {
	expr := `({
			chunks: [],
			recorder: null,
			streamPromise: null,
			request: function() {
				this.streamPromise = navigator.mediaDevices.getDisplayMedia({
					audio: false,
					video: {
						cursor: "always"
					}
				});
			},
			start: function() {
				this.chunks = [];
				return this.streamPromise.then(
					stream => {
						this.recorder = new MediaRecorder(stream, {mimeType: 'video/webm;codecs=vp9'});
						this.recorder.ondataavailable = (e) => {
							this.chunks.push(e.data);
						};
						this.recorder.start();
					}
				);
			},
			stop: function() {
				return new Promise((resolve, reject) => {
					this.recorder.onstop = function() {
						let blob = new Blob(this.chunks, {'type': 'video/webm'});
						var reader = new FileReader();
						reader.onload = () => {
							resolve(reader.result);
						}
						reader.readAsDataURL(blob);
					}.bind(this);
					this.recorder.stop();
				})
			}
		})
	`
	videoRecorder := &chrome.JSObject{}
	if err := tconn.Eval(ctx, expr, videoRecorder); err != nil {
		return nil, errors.Wrap(err, "failed to initialize video recorder")
	}
	sr := &ScreenRecorder{isRecording: false, videoRecorder: videoRecorder}

	// Request to share the screen.
	if err := sr.videoRecorder.Call(ctx, nil, `function() {this.request();}`); err != nil {
		return nil, errors.Wrap(err, "failed to request display media")
	}

	// Choose to record the entire desktop/screen with no audio.
	mediaview, err := FindWithTimeout(ctx, tconn, FindParams{Name: "Share your screen", ClassName: "DesktopMediaPickerDialogView"}, timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find media picker view")
	}
	defer mediaview.Release(ctx)

	if err := WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to wait for animation finished")
	}

	desktopView, err := mediaview.DescendantWithTimeout(ctx, FindParams{ClassName: "DesktopMediaSourceView", Role: RoleTypeButton}, 30*time.Second)
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

	shareButton, err := mediaview.DescendantWithTimeout(ctx, FindParams{Name: "Share", Role: RoleTypeButton}, timeout)
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

	return sr, nil
}

// Start creates a new media recorder and starts to record the screen. As long as ScreenRecorder
// is not recording, it can start to record again.
func (r *ScreenRecorder) Start(ctx context.Context) error {
	if r.isRecording == true {
		return errors.New("recorder already started")
	}

	if err := r.videoRecorder.Call(ctx, nil, `function() {this.start();}`); err != nil {
		return errors.Wrap(err, "failed to start to record screen")
	}
	r.isRecording = true

	return nil
}

// Stop ends the screen recording and stores the encoded base64 string.
func (r *ScreenRecorder) Stop(ctx context.Context) error {
	if r.isRecording == false {
		return errors.New("recorder hasn't started yet")
	}

	var result string
	if err := r.videoRecorder.Call(ctx, &result, `function() {return this.stop();}`); err != nil {
		return errors.Wrap(err, "failed to stop recording screen")
	}
	r.result = result
	r.isRecording = false

	return nil
}

// SaveInBytes saves the latest encoded string into a decoded bytes file.
func (r *ScreenRecorder) SaveInBytes(ctx context.Context, filepath string) error {
	parts := strings.Split(r.result, ",")
	if len(parts) < 2 {
		return errors.New("no content has been recorded. The recorder might have been stopped too soon")
	}

	// Decode base64 string.
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(parts[1]))
	buf := bytes.Buffer{}
	if _, err := buf.ReadFrom(reader); err != nil {
		return errors.Wrap(err, "failed to read from decoder")
	}
	if err := ioutil.WriteFile(filepath, buf.Bytes(), 0644); err != nil {
		return errors.Wrapf(err, "failed to dump bytes to %s", filepath)
	}
	return nil
}

// SaveInString saves the latest encoded string into a string file.
func (r *ScreenRecorder) SaveInString(ctx context.Context, filepath string) error {
	result := strings.Split(r.result, ",")[1]
	if err := ioutil.WriteFile(filepath, []byte(result), 0644); err != nil {
		return errors.Wrapf(err, "failed to dump string to %s", filepath)
	}
	return nil
}

// Release frees the reference to Javascript for this video recorder.
func (r *ScreenRecorder) Release(ctx context.Context) {
	if r.isRecording {
		r.Stop(ctx)
	}
	r.videoRecorder.Release(ctx)
}
