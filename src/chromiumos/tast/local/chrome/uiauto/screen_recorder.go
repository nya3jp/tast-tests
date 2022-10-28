// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uiauto

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/input"
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

type testingState interface {
	OutDir() string
	HasError() bool
}

func requestScreenShare(ctx context.Context, tconn *chrome.TestConn) (*ScreenRecorder, error) {
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
					this.videoTrack.stop();
				});
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

	return sr, nil
}

// NewScreenRecorder creates a ScreenRecorder.
// It only needs to create one ScreenRecorder during one test.
// It chooses the entire desktop as the media stream.
// Example:
//
//	  screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
//	  if err != nil {
//			s.Log("Failed to create ScreenRecorder: ", err)
//	  }
//
// To stop, save, and release the recorder:
//
//	defer uiauto.ScreenRecorderStopSaveRelease(...)
func NewScreenRecorder(ctx context.Context, tconn *chrome.TestConn) (*ScreenRecorder, error) {
	sr, err := requestScreenShare(ctx, tconn)

	if err != nil {
		return nil, err
	}

	// Choose to record the entire desktop/screen with no audio.
	ui := New(tconn)
	shareScreenDialog := nodewith.Name("Choose what to share").ClassName("DesktopMediaPickerDialogView")
	entireScreenTab := nodewith.Name("Entire Screen").Role(role.Tab).Ancestor(shareScreenDialog)
	firstDisplay := nodewith.Role(role.Button).Focusable().Ancestor(shareScreenDialog).First()
	// The share button becomes focusable after the entire desktop button is clicked.
	shareButton := nodewith.Name("Share").Role(role.Button).Ancestor(shareScreenDialog).Focusable()

	if err := Combine("start screen recorder through ui",
		ui.WithInterval(500*time.Millisecond).LeftClickUntil(entireScreenTab, ui.Exists(firstDisplay)),
		ui.WithInterval(500*time.Millisecond).LeftClickUntil(firstDisplay, ui.Exists(shareButton)),
		ui.LeftClick(shareButton),
	)(ctx); err != nil {
		return nil, err
	}

	return sr, nil
}

// NewWindowRecorder creates a ScreenRecorder, using a window as the media stream.
// The specific window can be chosen with the windowIndex parameter.
// For other information see the comment on the NewScreenRecorder function.
func NewWindowRecorder(ctx context.Context, tconn *chrome.TestConn, windowIndex int) (*ScreenRecorder, error) {
	sr, err := requestScreenShare(ctx, tconn)

	if err != nil {
		return nil, err
	}

	ui := New(tconn)
	shareScreenDialog := nodewith.Name("Choose what to share").ClassName("DesktopMediaPickerDialogView")
	windowTab := nodewith.Name("Window").Role(role.Tab).Ancestor(shareScreenDialog)
	windowButton := nodewith.Role(role.Button).Ancestor(shareScreenDialog).Nth(windowIndex)
	shareButton := nodewith.Name("Share").Role(role.Button).Ancestor(shareScreenDialog).Focusable()

	if err := Combine("start screen recorder through ui",
		ui.LeftClick(windowTab),
		ui.WithInterval(500*time.Millisecond).LeftClickUntil(windowButton, ui.Exists(shareButton)),
		ui.LeftClick(shareButton),
	)(ctx); err != nil {
		return nil, err
	}

	return sr, nil
}

// NewTabRecorder creates a ScreenRecorder, using a tab as the media stream.
// The specific tab can be chosen with the tabIndex parameter.
// For other information see the comment on the NewScreenRecorder function.
func NewTabRecorder(ctx context.Context, tconn *chrome.TestConn, tabIndex int) (*ScreenRecorder, error) {
	sr, err := requestScreenShare(ctx, tconn)

	if err != nil {
		return nil, err
	}

	ui := New(tconn)
	shareScreenDialog := nodewith.Name("Choose what to share").ClassName("DesktopMediaPickerDialogView")
	shareChromeTabOption := nodewith.NameRegex(regexp.MustCompile("(Chromium|Chrome) Tab")).Role(role.Tab).Ancestor(shareScreenDialog)
	tabButton := nodewith.Role(role.Row).Ancestor(shareScreenDialog).Nth(tabIndex)
	shareButton := nodewith.Name("Share").Role(role.Button).Ancestor(shareScreenDialog).Focusable()

	if err := Combine("start screen recorder through ui",
		ui.LeftClick(shareChromeTabOption),
		ui.WithInterval(500*time.Millisecond).LeftClickUntil(tabButton, ui.Exists(shareButton)),
		ui.LeftClick(shareButton),
	)(ctx); err != nil {
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
	testing.ContextLog(ctx, "Started screen recording")
	r.isRecording = true

	ui := New(tconn)
	closeNotificationButton := nodewith.Name("Notification close").Role(role.Button)
	messagePopupAlert := nodewith.ClassName("MessagePopupView").Role(role.AlertDialog)
	if err := ui.LeftClickUntil(closeNotificationButton, ui.WithInterval(time.Second).WaitUntilGone(messagePopupAlert))(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to dismiss screenshare notification popup, it likely didn't appear: ", err)
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
		return "", errors.Wrap(err, "failed to get frame status")
	}
	return result, nil
}

// Release frees the reference to Javascript for this video recorder.
func (r *ScreenRecorder) Release(ctx context.Context) {
	if r.isRecording {
		if err := r.Stop(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to stop screen recorder: ", err)
		}
	}
	if err := r.videoRecorder.Release(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to release screen recorder: ", err)
	}
}

// StopAndSaveOnError ends the screen recording and save it on error.
func (r *ScreenRecorder) StopAndSaveOnError(ctx context.Context, filepath string, hasError func() bool) {
	if err := r.Stop(ctx); err != nil {
		testing.ContextLogf(ctx, "Failed to stop recording: %s", err)
	} else if hasError() {
		// If there's an error, we want to wait long enough to see what happens
		// after the error. This allows you to see subtitles when the error has
		// occurred, and also happens to help in case something happens after
		// timing out.
		testing.Sleep(ctx, 2*time.Second)

		testing.ContextLogf(ctx, "Saving screen record to %s", filepath)
		if err := r.SaveInBytes(ctx, filepath); err != nil {
			testing.ContextLogf(ctx, "Failed to save screen record in bytes: %s", err)
		}
	}
	r.Release(ctx)
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

// RecordScreen records the screen for the duration of the function into recording.webm.
// For example, if you wanted to record your whole test, you would do the following:
// func MyTest(ctx context.Context, s *testing.State) {
//
//	cr := s.PreValue().(pre.PreData).Chrome
//	tconn := s.PreValue().(pre.PreData).TestAPIConn
//	uiauto.RecordScreen(ctx, s, tconn, func() {
//	<all your existing test code here>
//	})
//
// }
func RecordScreen(ctx context.Context, s testingState, tconn *chrome.TestConn, f func()) {
	recorder, err := NewScreenRecorder(ctx, tconn)
	if err != nil {
		testing.ContextLog(ctx, "Failed to start screen recording: ", err)
	}
	if recorder != nil {
		recorder.Start(ctx, tconn)
		defer func() {
			// If there's an error, we want to wait long enough to see what happens
			// after the error. This allows you to see subtitles when the error has
			// occurred, and also happens to help in case something happens after
			// timing out.
			if s.HasError() {
				testing.Sleep(ctx, time.Second*2)
			}
			ScreenRecorderStopSaveRelease(ctx, recorder, filepath.Join(s.OutDir(), "recording.webm"))
		}()
	}
	f()
}

// CreateAndStartScreenRecorder creates a ScreenRecorder and starts to record
// the screen. Handles errors by logging them via the context and returning nil.
// To handle errors manually, call CreateAndStartScreenRecorderWithError.
//
// Example usage:
//
//	recorder := uiauto.CreateAndStartScreenRecorder(ctx, tconn)
//	defer uiauto.StopAndSaveOnError(cleanupCtx, recorder, filepath.Join(s.OutDir(), "screen_recording.webm"), s.HasError)
func CreateAndStartScreenRecorder(ctx context.Context, tconn *chrome.TestConn) *ScreenRecorder {
	recorder, err := CreateAndStartScreenRecorderWithError(ctx, tconn)
	if err != nil {
		testing.ContextLog(ctx, "Failed to create and start the screen recorder: ", err)
	}
	return recorder
}

// CreateAndStartScreenRecorderWithError creates a ScreenRecorder and starts to
// record the screen. To handle errors automatically, call
// CreateAndStartScreenRecorder.
//
// Example usage:
//
//	recorder, err := uiauto.CreateAndStartScreenRecorderWithError(ctx, tconn)
//	if err != nil {
//		s.Log(ctx, "Failed to create and start the screen recorder: ", err)
//	}
//	defer uiauto.StopAndSaveOnError(cleanupCtx, recorder, filepath.Join(s.OutDir(), "screen_recording.webm"), s.HasError)
func CreateAndStartScreenRecorderWithError(ctx context.Context, tconn *chrome.TestConn) (*ScreenRecorder, error) {
	recorder, err := NewScreenRecorder(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create screen recorder")
	}
	if recorder != nil {
		if err := recorder.Start(ctx, tconn); err != nil {
			return nil, errors.Wrap(err, "failed to start screen recorder")
		}
	}
	return recorder, nil
}

// StopAndSaveOnError ends the screen recording, and saves it on error.
// Skips if the recorder is nil.
func StopAndSaveOnError(ctx context.Context, sr *ScreenRecorder, filepath string, hasError func() bool) {
	if sr != nil {
		sr.StopAndSaveOnError(ctx, filepath, hasError)
	}
}

// StartRecordFromKB starts screen record from keyboard.
// It clicks Ctrl+Shift+F5 then select to record the whole desktop.
// The caller should also call StopRecordFromKB to stop the screen recorder,
// and save the record file.
// Here is an example to call this method:
//
//	if err := uiauto.StartRecordFromKB(ctx, tconn, keyboard); err != nil {
//	    s.Log("Failed to start recording: ", err)
//	}
//
//	defer uiauto.StopRecordFromKBAndSaveOnError(ctx, tconn, s.HasError, s.OutDir())
func StartRecordFromKB(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, downloadsPath string) error {
	screenRecordBtn := nodewith.Name("Screen record").Role(role.ToggleButton)
	fullScreenBtn := nodewith.Name("Record full screen").Role(role.ToggleButton)
	desktop := nodewith.Role(role.Window).First()
	ui := New(tconn)

	files, err := ioutil.ReadDir(downloadsPath)
	if err != nil {
		return errors.Wrap(err, "failed to read files from Downloads")
	}
	expectNumber := len(files) + 1
	checkRecordFile := func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			files, err = ioutil.ReadDir(downloadsPath)
			if err != nil {
				return errors.Wrap(err, "failed to read files from Downloads")
			}
			if len(files) == expectNumber {
				return nil
			}
			return errors.Wrapf(err, "failed to check number of files, got %d, want %d", len(files), expectNumber)
		}, &testing.PollOptions{Timeout: 5 * time.Second})
	}
	return Combine("start screen record",
		kb.AccelAction("Ctrl+Shift+F5"),
		ui.LeftClick(screenRecordBtn),
		ui.LeftClick(fullScreenBtn),
		ui.LeftClick(desktop), // It needs to click any button to start, so clicking on the middle of the desktop.
		checkRecordFile,       // Check a new record file is created in Downloads.
	)(ctx)
}

// StopRecordFromKBAndSaveOnError stops the record started by StartRecordFromKB.
// If there is error, it copies the record file to the target dir .
// It also removes the record file from Downloads for cleanup.
func StopRecordFromKBAndSaveOnError(ctx context.Context, tconn *chrome.TestConn, hasError func() bool, dir, downloadsPath string) error {
	recordResult := nodewith.Name("Screen recording completed").Role(role.Alert)
	ui := New(tconn)
	if err := Combine("stop record",
		ui.LeftClick(ScreenRecordStopButton),
		ui.WaitUntilExists(recordResult))(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to stop recording: ", err)
		return err
	}

	return SaveRecordFromKBOnError(ctx, tconn, hasError, dir, downloadsPath)
}

// ScreenRecordStopButton is the button to stop recording the screen.
var ScreenRecordStopButton = nodewith.Name("Stop screen recording").Role(role.Button)

// SaveRecordFromKBOnError saves the recording from StartRecordFromKB.
// This can be used without StopRecordFromKBAndSaveOnError if the screen recording was stopped automatically (i.e. if the screen was locked).
func SaveRecordFromKBOnError(ctx context.Context, tconn *chrome.TestConn, hasError func() bool, dir, downloadsPath string) error {
	files, err := ioutil.ReadDir(downloadsPath)
	if err != nil {
		return errors.Wrap(err, "failed to read files from Downloads")
	}
	for _, f := range files {
		path := filepath.Join(downloadsPath, f.Name())
		if strings.HasSuffix(f.Name(), ".webm") {
			defer os.RemoveAll(path)
			if hasError() {
				if err := crash.MoveFilesToOut(ctx, dir, path); err != nil {
					return errors.Wrapf(err, "failed to copy records to %s", dir)
				}
				testing.ContextLogf(ctx, "Successfully copied the record file %s to %s", f.Name(), dir)
			}
		}
	}
	return nil
}
