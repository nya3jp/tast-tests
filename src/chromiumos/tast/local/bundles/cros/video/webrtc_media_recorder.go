// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCMediaRecorder,
		Desc:         "Checks MediaRecorder on local and remote streams",
		Contacts:     []string{"shenghao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "chrome_internal"},
		Data:         []string{"webrtc_media_recorder.html"},
		Timeout:      3 * time.Minute,
	})
}

// WebRTCMediaRecorder checks that MediaRecorder is able to record a local stream or a
// peer connection remote stream. It also checks the basic Media Recorder
// functions such as start, stop, pause, resume. The test fails if the media recorder
// cannot exercise these basic functions.
func WebRTCMediaRecorder(ctx context.Context, s *testing.State) {
	chromeArgs := []string{
		// "--use-fake-ui-for-media-stream" avoids the need to grant camera/microphone permissions.
		// "--use-fake-device-for-media-stream" feeds fake stream with specified fps to
		// getUserMedia() instead of live camera input.
		"--use-fake-ui-for-media-stream",
		"--use-fake-device-for-media-stream",
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Error(err, "Failed to connect to Chrome: ")
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, js := range []string{
		"testStartAndRecorderState()",
		"testStartStopAndRecorderState()",
		"testStartAndDataAvailable('video/webm; codecs=h264')",
		"testStartAndDataAvailable('video/webm; codecs=vp9')",
		"testStartAndDataAvailable('video/webm; codecs=vp8')",

		"testStartWithTimeSlice()",
		"testResumeAndRecorderState()",
		"testIllegalResumeThrowsDOMError()",
		"testResumeAndDataAvailable()",
		"testPauseAndRecorderState()",

		"testPauseStopAndRecorderState()",
		"testPausePreventsDataavailableFromBeingFired()",
		"testIllegalPauseThrowsDOMError()",
		"testIllegalStopThrowsDOMError()",
		"testIllegalStartInRecordingStateThrowsDOMError()",

		"testIllegalStartInPausedStateThrowsDOMError()",
		"testTwoChannelAudio()",
		"testIllegalRequestDataThrowsDOMError()",
		"testAddingTrackToMediaStreamFiresErrorEvent()",
		"testRemovingTrackFromMediaStreamFiresErrorEvent()",
	} {
		if err := runTest(ctx, cr, server, js); err != nil {
			s.Errorf("Test %v failed: %v", js, err)
		}
	}
}

func runTest(ctx context.Context, cr *chrome.Chrome, server *httptest.Server, js string) error {
	conn, err := cr.NewConn(ctx, server.URL+"/webrtc_media_recorder.html")
	if err != nil {
		return errors.Wrap(err, "Failed to open recorder page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "pageLoaded"); err != nil {
		return errors.Wrap(err, "timed out waiting for page loading")
	}

	if err := conn.Eval(ctx, js, nil); err != nil {
		return errors.Wrapf(err, "failed to call %v", js)
	}

	if err := conn.WaitForExpr(ctx, "testProgress"); err != nil {
		return errors.Wrap(err, "timed out waiting for test completion")
	}

	result := ""
	if err := conn.Eval(ctx, "result", &result); err != nil {
		return errors.Wrap(err, "failed to evaluate |result| ")
	}

	if result != "PASS" {
		return errors.New(fmt.Sprintf("test %v failed. result was %q; want %q", js, result, "PASS"))
	}
	return nil
}
