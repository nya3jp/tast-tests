// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package peerconnection provides common code for webrtc.* RTCPeerConnection tests.
package peerconnection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
)

// VerifyHWAcceleratorMode is the type of codec to verify hardware acceleration for.
type VerifyHWAcceleratorMode int

const (
	// VerifyHWEncoderUsed refers to WebRTC hardware accelerated video encoding.
	VerifyHWEncoderUsed VerifyHWAcceleratorMode = iota
	// VerifyHWDecoderUsed refers to WebRTC hardware accelerated video decoding.
	VerifyHWDecoderUsed
	// NoVerifyHWAcceleratorUsed means it doesn't matter if WebRTC uses any accelerated video.
	NoVerifyHWAcceleratorUsed
)

const (

	// LoopbackFile is the file containing the RTCPeerConnection loopback code.
	LoopbackFile = "loopback_peerconnection.html"

	// SimulcastAdapterName is the name of the RTC Stat value when simulcast encoding is used.
	SimulcastAdapterName = "SimulcastEncoderAdapter"
)

// RunRTCPeerConnection launches a loopback RTCPeerConnection and inspects that the
// VerifyHWAcceleratorMode codec is hardware accelerated if profile is not NoVerifyHWAcceleratorUsed.
func RunRTCPeerConnection(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, verifyMode VerifyHWAcceleratorMode, profile string, simulcast bool) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return errors.Wrap(err, "failed to set values for verbose logging")
	}
	defer vl.Close()

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL+"/"+LoopbackFile)
	if err != nil {
		return errors.Wrap(err, "failed to open video page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "timed out waiting for page loading")
	}

	if err := conn.Call(ctx, nil, "start", profile, simulcast); err != nil {
		return errors.Wrap(err, "error establishing connection")
	}

	if verifyMode != NoVerifyHWAcceleratorUsed {
		if err := checkForCodecImplementation(ctx, conn, verifyMode, simulcast); err != nil {
			return errors.Wrap(err, "checkForCodecImplementation() failed")
		}
	}
	return nil
}

type peerConnectionType bool

const (
	localPeerConnection  peerConnectionType = false
	remotePeerConnection peerConnectionType = true
)

// readRTCReport reads an RTCStat report of the given typ from the specified peer connection.
// The out can be an arbitrary struct whose members are 'json' tagged, so that they will be filled.
func readRTCReport(ctx context.Context, conn *chrome.Conn, pc peerConnectionType, typ string, out interface{}) error {
	return conn.Call(ctx, out, `async(isRemote, type) => {
	  const peerConnection = isRemote ? remotePeerConnection : localPeerConnection;
	  const stats = await peerConnection.getStats(null);
	  if (stats == null) {
	    throw new Error("getStats() failed");
	  }
	  for (const [_, report] of stats) {
	    if (report['type'] === type) {
	      return report;
	    }
	  }
	  throw new Error("Stat not found");
	}`, pc, typ)
}

// checkForCodecImplementation parses the RTCPeerConnection and verifies that it
// is using hardware acceleration for verifyMode. This method uses the
// RTCPeerConnection getStats() API [1].
// [1] https://w3c.github.io/webrtc-pc/#statistics-model
func checkForCodecImplementation(ctx context.Context, conn *chrome.Conn, verifyMode VerifyHWAcceleratorMode, isSimulcast bool) error {
	// See [1] and [2] for the statNames to use here. The values are browser
	// specific, for Chrome, "ExternalDecoder" and "{V4L2,Vaapi, etc.}VideoEncodeAccelerator"
	// means that WebRTC is using hardware acceleration and anything else
	// (e.g. "libvpx", "ffmpeg", "unknown") means it is not.
	// A SimulcastEncoderAdapter is actually a grouping of implementations, so it can read e.g.
	// "SimulcastEncoderAdapter (libvpx, VaapiVideoEncodeAccelerator, VaapiVideoEncodeAccelerator)"
	// (note that there isn't a SimulcastDecoderAdapter).
	//
	// [1] https://w3c.github.io/webrtc-stats/#dom-rtcinboundrtpstreamstats-decoderimplementation
	// [2] https://w3c.github.io/webrtc-stats/#dom-rtcoutboundrtpstreamstats-encoderimplementation
	expectedImpl := "EncodeAccelerator"
	readImpl := func(ctx context.Context) (string, error) {
		var out struct {
			Encoder string `json:"encoderImplementation"`
		}
		if err := readRTCReport(ctx, conn, localPeerConnection, "outbound-rtp", &out); err != nil {
			return "", err
		}
		return out.Encoder, nil
	}

	if verifyMode == VerifyHWDecoderUsed {
		expectedImpl = "ExternalDecoder"
		readImpl = func(ctx context.Context) (string, error) {
			var out struct {
				Decoder string `json:"decoderImplementation"`
			}
			if err := readRTCReport(ctx, conn, remotePeerConnection, "inbound-rtp", &out); err != nil {
				return "", err
			}
			return out.Decoder, nil
		}
	}

	// Poll getStats() to wait until {decoder,encoder}Implementation gets filled in:
	// RTCPeerConnection needs a few frames to start up encoding/decoding; in the
	// meantime it returns "unknown".
	const pollInterval = 100 * time.Millisecond
	const pollTimeout = 200 * pollInterval
	var impl string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		impl, err = readImpl(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to retrieve and/or parse RTCStatsReport")
		}
		if impl == "unknown" {
			return errors.New("getStats() didn't fill in the codec implementation (yet)")
		}
		// "ExternalEncoder" is the default value for encoder implementations
		// before filling the actual one, see b/162764016.
		if verifyMode == VerifyHWEncoderUsed && impl == "ExternalEncoder" {
			return errors.New("getStats() didn't fill in the encoder implementation (yet)")
		}
		return nil
	}, &testing.PollOptions{Interval: pollInterval, Timeout: pollTimeout}); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Implementation: ", impl)

	if !strings.Contains(impl, expectedImpl) {
		return errors.Errorf("expected implementation not found, got %v, looking for %v", impl, expectedImpl)
	}
	if verifyMode == VerifyHWEncoderUsed && isSimulcast && !strings.HasPrefix(impl, SimulcastAdapterName) {
		return errors.Errorf("simulcast adapter not found, got %v, looking for %v", impl, SimulcastAdapterName)
	}

	return nil
}

// DataFiles returns a list of required files that tests that use this package
// should include in their Data fields.
func DataFiles() []string {
	return []string{
		"loopback_peerconnection.js",
		"third_party/blackframe.js",
		"third_party/munge_sdp.js",
		"third_party/sdp/sdp.js",
		"third_party/simulcast/simulcast.js",
		"third_party/ssim.js",
	}
}
