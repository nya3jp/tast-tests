// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package peerconnection provides common code for webrtc.* RTCPeerConnection tests.
package peerconnection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
)

// VerifyDecoderMode is how to verify decoder implementation.
type VerifyDecoderMode int

const (
	// VerifyHWDecoderUsed refers to WebRTC hardware accelerated video decoding.
	VerifyHWDecoderUsed VerifyDecoderMode = iota
	// VerifySWDecoderUsed refers to WebRTC software video decoding.
	VerifySWDecoderUsed
	// NoVerifyDecoderMode means it doesn't matter if WebRTC uses any video decoder.
	NoVerifyDecoderMode
)

// VerifyEncoderMode is how to verify encoder implementation.
type VerifyEncoderMode int

const (
	// VerifyHWEncoderUsed refers to WebRTC hardware accelerated video encoding.
	VerifyHWEncoderUsed VerifyEncoderMode = iota
	// VerifySWEncoderUsed refers to WebRTC software video encoding.
	VerifySWEncoderUsed
	// NoVerifyEncoderMode means it doesn't matter if WebRTC uses any video encoder.
	NoVerifyEncoderMode
)

// DisplayMediaType represents displaySurface property in displayMedia constraints.
// See https://w3c.github.io/mediacapture-screen-share/#dom-displaycapturesurfacetype.
type DisplayMediaType string

const (
	// CaptureMonitor is to capture an entire screen.
	CaptureMonitor DisplayMediaType = "monitor"
	// CaptureWindow is to capture a window.
	CaptureWindow = "window"
	// CaptureTab is to capture tab in a browser.
	CaptureTab = "browser"
)

const (

	// LoopbackFile is the file containing the RTCPeerConnection loopback code.
	LoopbackFile = "loopback_peerconnection.html"

	// SimulcastAdapterName is the name of the RTC Stat value when simulcast encoding is used.
	SimulcastAdapterName = "SimulcastEncoderAdapter"
)

// RTCTestParams is used to describe the config used to run RTCPeerConnectionPerf.
type RTCTestParams struct {
	verifyDecoderMode VerifyDecoderMode // The verification mode for decoder.
	verifyEncoderMode VerifyEncoderMode // The verification mode for encoder.
	profile           string            // Codec to try, e.g. VP8, VP9.
	streamWidth       int               // Width of video to be sent in the peerconnection.
	streamHeight      int               // Height of video to be sent in the peerconnection.
	// ScalableVideoCodec "scalabilityMode" identifier.
	// https://www.w3.org/TR/webrtc-svc/#scalabilitymodes
	svc string
	// The number of video streams in simulcast.
	simulcasts int
	// If non-empty, the media to send through the RTC connection will be obtained
	// using getDisplayMedia() and the value corresponds to the surface type. If
	// empty, the media to send will be obtained using getUserMedia().
	displayMediaType DisplayMediaType

	// Below are used only in performance tests.
	videoGridDimension int    // Dimension of the grid in which to embed the RTCPeerConnection <video>.
	videoGridFile      string // Name of the video file to fill up the grid with, if needed.
	// The array each element of which is true iff -th smaller resolution should be a hardware encoder implementation.
	simulcastHWEncs []bool
}

// RunRTCPeerConnection launches a loopback RTCPeerConnection and inspects that the
// VerifyHWAcceleratorMode codec is hardware accelerated if profile is not NoVerifyHWAcceleratorUsed.
func RunRTCPeerConnection(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, verifyDecoderMode VerifyDecoderMode, verifyEncoderMode VerifyEncoderMode, profile string, simulcast bool, svc string, displayMediaType DisplayMediaType) error {
	if simulcast && svc != "" {
		return errors.New("|simulcast| and |svc| cannot be set simultaneously")
	}
	if displayMediaType != "" && (simulcast || svc != "") {
		return errors.New("Screen capture can't be used with simulcast or SVC")
	}
	if verifyDecoderMode != NoVerifyDecoderMode && verifyEncoderMode != NoVerifyEncoderMode {
		return errors.New("Decoder and encoder implementation cannot be verified simultaneously")
	}

	vl, err := logging.NewVideoLogger()
	if err != nil {
		return errors.Wrap(err, "failed to set values for verbose logging")
	}
	defer vl.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}

	if _, err := display.GetInternalInfo(ctx, tconn); err == nil {
		// The device has an internal display.
		// For consistency across test runs, ensure that the device is in landscape-primary orientation.
		if err = graphics.RotateDisplayToLandscapePrimary(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to set display to landscape-primary orientation")
		}
	}

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

	const simulcastStreams = 3
	simulcasts := 0
	if simulcast {
		simulcasts = simulcastStreams
	}
	if err := conn.Call(ctx, nil, "start", profile, simulcasts, svc, displayMediaType); err != nil {
		return errors.Wrap(err, "error establishing connection")
	}

	simulcastHWEncs := make([]bool, simulcasts)
	for i := 0; i < simulcasts; i++ {
		simulcastHWEncs[i] = true
	}
	if err := verifyDecoderImplementation(ctx, conn, verifyDecoderMode); err != nil {
		return err
	}
	if err := verifyEncoderImplementation(ctx, conn, verifyEncoderMode, simulcastHWEncs); err != nil {
		return err
	}

	return nil
}

func verifyDecoderImplementation(ctx context.Context, conn *chrome.Conn, verifyDecoderMode VerifyDecoderMode) error {
	if verifyDecoderMode == NoVerifyDecoderMode {
		return nil
	}

	decImplName, hwDecoderUsed, err := getCodecImplementation(ctx, conn /*decode=*/, true)
	if err != nil {
		return errors.Wrap(err, "failed to get decoder implementation name")
	}
	if verifyDecoderMode == VerifyHWDecoderUsed && !hwDecoderUsed {
		return errors.Errorf("hardware decode accelerator wasn't used, got %s", decImplName)
	}
	if verifyDecoderMode == VerifySWDecoderUsed && hwDecoderUsed {
		return errors.Errorf("software decode wasn't used, got %s", decImplName)
	}
	return nil
}

func verifyEncoderImplementation(ctx context.Context, conn *chrome.Conn, verifyEncoderMode VerifyEncoderMode, simulcastHWEncs []bool) error {
	if verifyEncoderMode == NoVerifyEncoderMode {
		return nil
	}

	encImplName, hwEncoderUsed, err := getCodecImplementation(ctx, conn, false)
	if err != nil {
		return errors.Wrap(err, "failed to get encoder implementation name")
	}
	if len(simulcastHWEncs) > 1 {
		return checkSimulcastEncImpl(encImplName, simulcastHWEncs)
	}

	if verifyEncoderMode == VerifyHWEncoderUsed && !hwEncoderUsed {
		return errors.Errorf("hardware encode accelerator wasn't used, got %s", encImplName)
	}
	if verifyEncoderMode == VerifySWEncoderUsed && hwEncoderUsed {
		return errors.Errorf("software encode wasn't used, got %s", encImplName)
	}
	return nil
}

type peerConnectionType bool

const (
	localPeerConnection  peerConnectionType = false
	remotePeerConnection peerConnectionType = true
)

// readRTCReport reads an RTCStat report of the given typ from the specified peer connection.
// Since there are multiple outbound-rtp in the case of simulcast, the stat is selected whose frame height is the largest.
// The out can be an arbitrary struct whose members are 'json' tagged, so that they will be filled.
func readRTCReport(ctx context.Context, conn *chrome.Conn, pc peerConnectionType, typ string, out interface{}) error {
	return conn.Call(ctx, out, `async(isRemote, type) => {
	  const peerConnection = isRemote ? remotePeerConnection : localPeerConnection;
	  const stats = await peerConnection.getStats(null);
	  if (stats == null) {
	    throw new Error("getStats() failed");
	  }
      var R = null;
	  for (const [_, report] of stats) {
	    if (report['type'] === type &&
            (!R || R['frameHeight'] < report['frameHeight'])) {
          R = report;
	    }
	  }
      if (R !== null) {
        return R;
      }
	  throw new Error("Stat not found");
	}`, pc, typ)
}

// getCodecImplementation parses the RTCPeerConnection and returns the implementation name and whether it is
// a hardware implementation. If decode is true, this returns decoder implementation and otherwise encoder implementation.
// This method uses the RTCPeerConnection getStats() API [1].
// [1] https://w3c.github.io/webrtc-pc/#statistics-model
func getCodecImplementation(ctx context.Context, conn *chrome.Conn, decode bool) (string, bool, error) {
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
	hwImplName := "EncodeAccelerator"
	readImpl := func(ctx context.Context) (string, error) {
		var out struct {
			Encoder string `json:"encoderImplementation"`
		}
		if err := readRTCReport(ctx, conn, localPeerConnection, "outbound-rtp", &out); err != nil {
			return "", err
		}
		return out.Encoder, nil
	}

	if decode {
		hwImplName = "ExternalDecoder"
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
		if impl == "ExternalEncoder" {
			return errors.New("getStats() didn't fill in the encoder implementation (yet)")
		}
		return nil
	}, &testing.PollOptions{Interval: pollInterval, Timeout: pollTimeout}); err != nil {
		return "", false, err
	}
	testing.ContextLog(ctx, "Implementation: ", impl)

	isHWImpl := strings.Contains(impl, hwImplName)
	return impl, isHWImpl, nil
}

// checkSimulcastEncImpl checks that the given implName is used in given simulcast scenario.
// isImplHWInAdapter[i] stands for whether i-th stream in the simulcast is expected to be produced by a hardware encoder.
func checkSimulcastEncImpl(implName string, isImplHWInAdapter []bool) error {
	isAllSWEnc := true
	for _, isHW := range isImplHWInAdapter {
		if isHW {
			isAllSWEnc = false
			break
		}
	}
	// If all the streams in the simulcast are produced by a software encoder.
	// The implementation name is libvpx because a libvpx encoder supports simulcast.
	if isAllSWEnc {
		if implName == "libvpx" {
			return nil
		}
		return errors.Errorf("unexpected simulcast encoder adapter name: %s", implName)
	}

	// If the streams in the simulcast are produced by software and hardware encoders or
	// only hardware encoders, SimulcastEncoderAdapter is used to bundle the streams.
	// The implementation name is like SimulcastEncoderAdapter(libvpx, VaapiVideoEncodeAccelerator, VaapiVideoEncodeAccelerator).
	implName = strings.ReplaceAll(implName, " ", "")
	re := regexp.MustCompile(`SimulcastEncoderAdapter\(([\S, ]+)\)`)
	inStrs := re.FindStringSubmatch(implName)
	if inStrs == nil {
		return errors.Errorf("failed to extract internal encoder names: %s", implName)
	}

	implNames := strings.Split(inStrs[1], ",")
	if len(implNames) != len(isImplHWInAdapter) {
		return errors.Errorf("the number of simulcast streams mismatches: got %d (%v), expected %d", len(implNames), implNames, len(isImplHWInAdapter))
	}

	const hwImplName = "EncodeAccelerator"
	for i, implName := range implNames {
		if isImplHWInAdapter[i] != strings.Contains(implName, hwImplName) {
			return errors.Errorf("unexpected implementations within simulcast adapter: %v", implNames)
		}
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
