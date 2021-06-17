// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uiauto

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
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

// NewScreenRecorder creates a ScreenRecorder.
// It only needs to create one ScreenRecorder during one test.
// It chooses the entire desktop as the media stream.
// Example:
//
//   screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
//   if err != nil {
//		s.Log("Failed to create ScreenRecorder: ", err)
//   }
//
// To stop, save, and release the recorder:
//    defer uiauto.ScreenRecorderStopSaveRelease(...)
//
func NewScreenRecorder(ctx context.Context, tconn *chrome.TestConn) (*ScreenRecorder, error) {
	expr := `({
			chunks: [],
			recorder: null,
			streamPromise: null,
			videoTrack: null,
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
						this.videoTrack = stream.getVideoTracks()[0];
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
			},
			frameStatus: function() {
				return new Promise((resolve, reject) => {
					const imageCapture = new ImageCapture(this.videoTrack)

					imageCapture.grabFrame()
					.then(function(imageBitmap) {
						if (imageBitmap.width) {
							resolve('Success');
						}
					})
					.catch(function(error) {
						resolve('Fail');
					});
				});
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
	ui := New(tconn)
	shareScreenDialog := nodewith.Name("Choose what to share").ClassName("DesktopMediaPickerDialogView")
	entireDesktopButton := nodewith.ClassName("DesktopMediaSourceView").Role(role.Button).Ancestor(shareScreenDialog)
	shareButton := nodewith.Name("Share").Role(role.Button).Ancestor(shareScreenDialog)

	if err := Combine("start screen recorder through ui", ui.LeftClick(entireDesktopButton), ui.LeftClick(shareButton))(ctx); err != nil {
		return nil, err
	}

	return sr, nil
}

// Start creates a new media recorder and starts to record the screen. As long as ScreenRecorder
// is not recording, it can start to record again.
func (r *ScreenRecorder) Start(ctx context.Context, tconn *chrome.TestConn) error {
	if r.isRecording == true {
		return errors.New("recorder already started")
	}

	if err := r.videoRecorder.Call(ctx, nil, `function() {this.start();}`); err != nil {
		return errors.Wrap(err, "failed to start to record screen")
	}
	r.isRecording = true

	ui := New(tconn)
	closeNotificationButton := nodewith.Name("Notification close").Role(role.Button)
	messagePopupAlert := nodewith.ClassName("MessagePopupView").Role(role.AlertDialog)
	if err := Combine("close notification and wait for it to disappear",
		ui.WithInterval(1*time.Second).LeftClick(closeNotificationButton),
		ui.WaitUntilGone(messagePopupAlert))(ctx); err != nil {
		return err
	}
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

// FrameStatus returns the status of the frame being recorded.
func (r *ScreenRecorder) FrameStatus(ctx context.Context) (string, error) {
	var result string
	if err := r.videoRecorder.Call(ctx, &result, `function() {return this.frameStatus();}`); err != nil {
		return "", errors.Wrap(err, "failed to get frame status: ")
	}
	return result, nil
}

// Release frees the reference to Javascript for this video recorder.
func (r *ScreenRecorder) Release(ctx context.Context) {
	if r.isRecording {
		r.Stop(ctx)
	}
	r.videoRecorder.Release(ctx)
}

// ScreenRecorderStopSaveRelease stops, saves and releases the screen recorder.
func ScreenRecorderStopSaveRelease(ctx context.Context, r *ScreenRecorder, fileName string) {
	if r != nil {
		if err := r.Stop(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to stop recording: %s", err)
		} else {
			testing.ContextLogf(ctx, "Saving screen record to %s", fileName)
			if err := r.SaveInBytes(ctx, fileName); err != nil {
				testing.ContextLogf(ctx, "Failed to save screen record in bytes: %s", err)
			}
		}
		r.Release(ctx)
	}
}
