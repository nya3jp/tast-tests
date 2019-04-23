// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/mediarecorder"
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
		Timeout:      10 * time.Minute,
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

	type FuncAndParam struct {
		function string
		param    string
	}

	funcAndParams := []FuncAndParam{
		{function: "testStartAndRecorderState", param: ""},
		{function: "testStartStopAndRecorderState", param: ""},
		{function: "testStartAndDataAvailable", param: "\"video/webm; codecs=h264\""},
		{function: "testStartAndDataAvailable", param: "\"video/webm; codecs=vp9\""},
		{function: "testStartAndDataAvailable", param: "\"video/webm; codecs=vp8\""},

		{function: "testStartWithTimeSlice", param: ""},
		{function: "testResumeAndRecorderState", param: ""},
		{function: "testIllegalResumeThrowsDOMError", param: ""},
		{function: "testResumeAndDataAvailable", param: ""},
		{function: "testPauseAndRecorderState", param: ""},

		{function: "testPauseStopAndRecorderState", param: ""},
		{function: "testPausePreventsDataavailableFromBeingFired", param: ""},
		{function: "testIllegalPauseThrowsDOMError", param: ""},
		{function: "testIllegalStopThrowsDOMError", param: ""},
		{function: "testIllegalStartInRecordingStateThrowsDOMError", param: ""},

		{function: "testIllegalStartInPausedStateThrowsDOMError", param: ""},
		{function: "testTwoChannelAudio", param: ""},
		{function: "testIllegalRequestDataThrowsDOMError", param: ""},
		{function: "testAddingTrackToMediaStreamFiresErrorEvent", param: ""},
		{function: "testRemovingTrackFromMediaStreamFiresErrorEvent", param: ""},
	}

	for _, f := range funcAndParams {
		if err := mediarecorder.LaunchTest(ctx, s.DataFileSystem(), chromeArgs, f.function, f.param); err != nil {
			s.Errorf("test %v(%v) failed: %v", f.function, f.param, err)
		}
	}
}
