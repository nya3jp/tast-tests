// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaRecorderAPI,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies the MediaRecorder API",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		SoftwareDeps: []string{"chrome", "proprietary_codecs"},
		Data:         []string{"media_recorder.html", "media_recorder.js"},
		Fixture:      "chromeVideoWithFakeWebcam",
		Timeout:      3 * time.Minute,
	})
}

// MediaRecorderAPI verifies the MediaRecorder API, e.g. functions such as
// start, stop, pause, resume. The test fails if the media recorder cannot
// exercise these basic functions.
func MediaRecorderAPI(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL+"/media_recorder.html")
	if err != nil {
		s.Error(err, "failed to open recorder page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	runTest := func(js string) error {
		s.Logf("Running %s", js)
		if err := conn.Eval(ctx, js, nil); err != nil {
			return errors.Wrap(err, "failed to evaluate test function")
		}
		return nil
	}

	for _, js := range []string{
		// Test start and stop.
		"testStartAndRecorderState()",
		"testStartStopAndRecorderState()",
		"testStartAndDataAvailable('video/webm; codecs=h264')",
		"testStartAndDataAvailable('video/webm; codecs=vp9')",
		"testStartAndDataAvailable('video/webm; codecs=vp8')",
		"testStartWithTimeSlice()",

		// Test resume and pause.
		"testResumeAndRecorderState()",
		"testResumeAndDataAvailable()",
		"testPauseAndRecorderState()",
		"testPauseStopAndRecorderState()",
		"testPausePreventsDataavailableFromBeingFired()",

		// Test illegal operations handling.
		"testIllegalResumeThrowsDOMError()",
		"testIllegalPauseThrowsDOMError()",
		"testIllegalStopThrowsDOMError()",
		"testIllegalStartInRecordingStateThrowsDOMError()",
		"testIllegalStartInPausedStateThrowsDOMError()",
		"testIllegalRequestDataThrowsDOMError()",

		"testTwoChannelAudio()",
		"testAddingTrackToMediaStreamFiresErrorEvent()",
		"testRemovingTrackFromMediaStreamFiresErrorEvent()",
	} {
		if err := runTest(js); err != nil {
			s.Errorf("%v failed: %v", js, err)
		}
	}
}
