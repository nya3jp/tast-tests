// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

// RunWebRTCVideo tests HW decoders/encoders are used in WebRTC communication.
// This artificially performs WebRTC communication with streamName on loopback.html.
// The test fails if bucketValue on histogramName does not count up.
func RunWebRTCVideo(ctx context.Context, s *testing.State, streamName, histogramName string, bucketValue int64) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := openWebRTCPageAndCheckBucket(ctx, s.DataFileSystem(), s.DataPath(streamName), histogramName, bucketValue); err != nil {
		s.Fatal("Failed: ", err)
	}
}

// openWebRTCPageAndCheckBucket opens video/data/loopback.html and communicates via WebRTC in a fake way. The stream on WebRTC is streamFile.
// It checks bucketValue on histogramName counts up in the end of the test.
func openWebRTCPageAndCheckBucket(ctx context.Context, fileSystem http.FileSystem, streamFile, histogramName string, bucketValue int64) error {
	chromeArgs := webrtc.ChromeArgsWithCameraInput(streamFile, true)
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	initHistogram, err := metrics.GetHistogram(ctx, cr, histogramName)
	if err != nil {
		return errors.Wrap(err, "failed to get initial histogram")
	}
	testing.ContextLogf(ctx, "Initial %s histogram: %v", histogramName, initHistogram.Buckets)

	conn, err := cr.NewConn(ctx, server.URL+"/"+webrtc.LoopbackPage)
	if err != nil {
		return errors.Wrap(err, "failed to open video page")
	}
	defer conn.Close()
	// Close the tab to stop loopback after test.
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "streamReady"); err != nil {
		return errors.Wrap(err, "timed out waiting for stream ready")
	}

	if err := checkError(ctx, conn); err != nil {
		return err
	}

	histogramDiff, err := metrics.WaitForHistogramUpdate(ctx, cr, histogramName, initHistogram, 15*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed getting histogram diff")
	}

	if len(histogramDiff.Buckets) > 1 {
		return errors.Wrapf(err, "unexpected histogram update: %v", histogramDiff)
	}

	bucket := histogramDiff.Buckets[0]
	// Expected histogram is [bucketValue, bucketValue+1, 1].
	if bucket.Min != bucketValue || bucket.Max != bucketValue+1 || bucket.Count != 1 {
		return errors.Wrapf(err, "unexpected histogram update: %v", bucket)
	}

	return nil
}

func checkError(ctx context.Context, conn *chrome.Conn) error {
	var getUserMediaError, gotLocalDescriptionError, gotRemoteDescriptionError string
	if err := conn.Eval(ctx, "getUserMediaError", &getUserMediaError); err != nil {
		return err
	}
	if err := conn.Eval(ctx, "gotLocalDescriptionError", &gotLocalDescriptionError); err != nil {
		return err
	}
	if err := conn.Eval(ctx, "gotRemoteDescriptionError", &gotRemoteDescriptionError); err != nil {
		return err
	}
	if getUserMediaError != "" || gotLocalDescriptionError != "" || gotRemoteDescriptionError != "" {
		return errors.Errorf("error in JS functions: getUserMediaError=%s, gotLocalDescriptionError=%s, gotRemoteDescriptionError=%s", getUserMediaError, gotLocalDescriptionError, gotRemoteDescriptionError)
	}
	return nil
}
