// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package video provides common code for webrtc.* tests related to video.
package video

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

// RunGetUserMedia tests that the HW JPEG decoder is used in a GetUserMedia().
// The test fails if bucketValue on histogramName does not count up.
func RunGetUserMedia(ctx context.Context, s *testing.State, getUserMediaFilename, streamName, histogramName string, bucketValue int64) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := openPageAndCheckBucket(ctx, s.DataFileSystem(), getUserMediaFilename, s.DataPath(streamName), histogramName, bucketValue); err != nil {
		s.Fatal("Failed: ", err)
	}
}

// openPageAndCheckBucket opens getUserMediaFilename, and uses GetUserMedia() to
// stream streamFile. Then it verifies that bucketValue on histogramName counts
// up in the end of the test.
func openPageAndCheckBucket(ctx context.Context, fileSystem http.FileSystem, getUserMediaFilename, streamFile, histogramName string, bucketValue int64) error {
	chromeArgs := webrtc.ChromeArgsWithFileCameraInput(streamFile, true)
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

	conn, err := cr.NewConn(ctx, server.URL+"/"+getUserMediaFilename)
	if err != nil {
		return errors.Wrapf(err, "failed to open page %s", getUserMediaFilename)
	}
	defer conn.Close()
	// Close the tab to stop loopback after test.
	defer conn.CloseTarget(ctx)

	const getUserMediaCode = `new Promise((resolve, reject) => {
			const constraints = { audio: false, video: true };

			navigator.mediaDevices.getUserMedia(constraints)
			.then(stream => {
                            document.getElementById('localVideo').srcObject = stream;
                            resolve();
                        })
			.catch(reject);
		});`
	if err := conn.EvalPromise(ctx, getUserMediaCode, nil); err != nil {
		return errors.Wrap(err, "getUserMedia() establishment failed")
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
