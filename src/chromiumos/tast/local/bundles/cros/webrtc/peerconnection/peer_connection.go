// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package peerconnection provides common code for webrtc.* RTCPeerConnection tests.
package peerconnection

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

// CodecType is the type of codec to check for.
type CodecType int

const (
	// Encoding refers to WebRTC video encoding.
	Encoding CodecType = iota
	// Decoding refers to WebRTC video decoding.
	Decoding
)

// RunPeerConnection launches a loopback RTCPeerConnection and inspects that the
// CodecType codec is hardware accelerated.
func RunPeerConnection(ctx context.Context, s *testing.State, codecType CodecType, profile string) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	chromeArgs := webrtc.ChromeArgsWithFakeCameraInput(true)
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromeArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL+"/"+webrtc.LoopbackPage)
	if err != nil {
		s.Fatal("Failed to open video page: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		s.Fatal("Timed out waiting for page loading: ", err)
	}

	if err := conn.EvalPromise(ctx, fmt.Sprintf("start(%q)", profile), nil); err != nil {
		s.Fatal("Error establishing connection: ", err)
	}

	if err := checkForCodecImplementation(ctx, s, conn, codecType); err != nil {
		s.Fatal("checkForCodecImplementation() failed: ", err)
	}
}

// checkForCodecImplementation parses the RTCPeerConnection and verifies that it
// is using hardware acceleration for codecType. This method uses the
// RTCPeerConnection getStats() API [1].
// [1] https://w3c.github.io/webrtc-pc/#statistics-model
func checkForCodecImplementation(ctx context.Context, s *testing.State, conn *chrome.Conn, codecType CodecType) error {
	// See [1] and [2] for the statNames to use here. The values are browser
	// specific, for Chrome, "External{Deco,Enco}der" means that WebRTC is using
	// hardware acceleration and anything else (e.g. "libvpx", "ffmpeg",
	// "unknown") means it is not.
	// [1] https://w3c.github.io/webrtc-stats/#dom-rtcinboundrtpstreamstats-decoderimplementation
	// [2] https://w3c.github.io/webrtc-stats/#dom-rtcoutboundrtpstreamstats-encoderimplementation
	statName := "encoderImplementation"
	peerConnectionName := "localPeerConnection"
	expectedImplementation := "ExternalEncoder"
	if codecType == Decoding {
		statName = "decoderImplementation"
		peerConnectionName = "remotePeerConnection"
		expectedImplementation = "ExternalDecoder"
	}

	parseStatsJS :=
		fmt.Sprintf(`new Promise(function(resolve, reject) {
			%s.getStats(null).then(stats => {
				if (stats == null) {
					reject("getStats() failed");
					return;
				}
				stats.forEach(report => {
					Object.keys(report).forEach(statName => {
						if (statName === '%s') {
							resolve(report[statName]);
							return;
						}
					})
				})
				reject("%s not found");
			});
		})`, peerConnectionName, statName, statName)

	// Poll getStats() to wait until expectedImplementation gets filled in:
	// RTCPeerConnection needs a few frames to start up encoding/decoding; in the
	// meantime it returns "unknown".
	const pollInterval = 100 * time.Millisecond
	const pollTimeout = 200 * pollInterval
	var implementation string
	err := testing.Poll(ctx,
		func(ctx context.Context) error {
			if err := conn.EvalPromise(ctx, parseStatsJS, &implementation); err != nil {
				return errors.Wrap(err, "failed to retrieve and/or parse RTCStatsReport")
			}
			if implementation == "unknown" {
				return errors.New("getStats() didn't fill in the codec implementation (yet)")
			}
			return nil
		}, &testing.PollOptions{Interval: pollInterval, Timeout: pollTimeout})

	if err != nil {
		return err
	}
	s.Logf("%s: %s", statName, implementation)

	if implementation != expectedImplementation {
		return errors.Errorf("unexpected implementation, got %v, expected %v", implementation, expectedImplementation)
	}
	return nil
}
