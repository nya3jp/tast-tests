// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mediarecorder provides common code for video.MediaRecorder tests.
package mediarecorder

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/testing"
)

// VerifyEncodeAccelUsed checks whether HW encode is used for given codec when running
// MediaRecorder.
func VerifyEncodeAccelUsed(ctx context.Context, s *testing.State, codec videotype.Codec) {
	chromeArgs := []string{
		logging.ChromeVmoduleFlag(),
		// See https://webrtc.org/testing/
		// "--use-fake-device-for-media-stream" avoids the need to grant camera/microphone permissions.
		// "--use-fake-ui-for-media-stream" feeds a test pattern to getUserMedia() instead of live camera input.
		"--use-fake-device-for-media-stream",
		"--use-fake-ui-for-media-stream",
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	initHistogram, err := metrics.GetHistogram(ctx, cr, constants.MediaRecorderVEAUsed)
	if err != nil {
		s.Fatal("Failed to get initial histogram: ", err)
	}

	conn, err := cr.NewConn(ctx, server.URL+"/loopback_media_recorder.html")
	if err != nil {
		s.Fatal("Failed to open recorder page: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "pageLoaded"); err != nil {
		s.Fatal("Timed out waiting for page loading: ", err)
	}

	startRecordJS := fmt.Sprintf("startRecording(%q)", codec)
	if err := conn.EvalPromise(ctx, startRecordJS, nil); err != nil {
		s.Fatalf("Failed to evaluate %v: %v", startRecordJS, err)
	}

	histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, cr, constants.MediaRecorderVEAUsed, initHistogram, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to get histogram diff: ", err)
	}

	if len(histogramDiff.Buckets) > 1 {
		s.Fatal("Unexpected histogram update: ", histogramDiff)
	}

	bucket := histogramDiff.Buckets[0]
	bucketValue := int64(constants.MediaRecorderVEAUsedSuccess)
	// Expected histogram is [bucketValue, bucketValue+1, 1].
	if bucket.Min != bucketValue || bucket.Max != bucketValue+1 || bucket.Count != 1 {
		s.Errorf("Unexpected histogram update: %v, expecting [%d, %d, 1]", bucket, bucketValue, bucketValue+1)
	}
}
