// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webcodecs

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

type hardwareAcceleration string

const (
	hardware hardwareAcceleration = "require"
	software hardwareAcceleration = "deny"
	noOption hardwareAcceleration = "allow"
)

type sourceType string

const (
	camera sourceType = "camera"
	canvas sourceType = "capture"
)

type testArgs struct {
	codec   videotype.Codec
	encoder hardwareAcceleration
	decoder hardwareAcceleration
	source  sourceType
}

const commonJS = "webcodecs_common.js"
const encodeDecodeHTML = "encode-decode.html"

func init() {
	testing.AddTest(&testing.Test{
		Func: EncodeDecode,
		Desc: "Verifies that WebCodecs API works, maybe verifying use of a hardware accelerator",
		Contacts: []string{
			"hiroh@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{encodeDecodeHTML, commonJS},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Fixture:      "chromeWebCodecsWithFakeWebcam",
		Params: []testing.Param{{
			Name:              "h264_sw",
			Val:               testArgs{codec: videotype.H264, encoder: software, decoder: software, source: camera},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
		}, {
			Name:              "h264_sw_capture",
			Val:               testArgs{codec: videotype.H264, encoder: software, decoder: software, source: canvas},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
		}, {
			Name: "vp8_sw",
			Val:  testArgs{codec: videotype.VP8, encoder: software, decoder: software, source: camera},
		}, {
			Name: "vp9_sw",
			Val:  testArgs{codec: videotype.VP9, encoder: software, decoder: software, source: camera},
		}, {
			Name:              "h264_hw_encoding",
			Val:               testArgs{codec: videotype.H264, encoder: hardware, decoder: software, source: camera},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264, "proprietary_codecs"},
		}, {
			Name:              "vp8_hw_encoding",
			Val:               testArgs{codec: videotype.VP8, encoder: hardware, decoder: software, source: camera},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP8},
		}, {
			Name:              "vp9_hw_encoding",
			Val:               testArgs{codec: videotype.VP9, encoder: hardware, decoder: software, source: camera},
			ExtraSoftwareDeps: []string{caps.HWEncodeVP9},
		},
		},
	})
}

func EncodeDecode(ctx context.Context, s *testing.State) {
	if err := RunEncodeDecode(ctx, s.FixtValue().(*chrome.Chrome),
		s.DataFileSystem(), s.Param().(testArgs)); err != nil {
		s.Error("Failed to run RunEncodeDecode: ", err)
	}
}

func RunEncodeDecode(ctx context.Context, cr *chrome.Chrome, fileSystem http.FileSystem, args testArgs) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return errors.Wrap(err, "failed to set values for verbose logging")
	}
	defer vl.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}
	defer tconn.Close()

	if _, err := display.GetInternalInfo(ctx, tconn); err == nil {
		// The device has an internal display.
		// For consistency across test runs, ensure that the device is in landscape-primary orientation.
		if err = graphics.RotateDisplayToLandscapePrimary(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to set display to landscape-primary orientation")
		}
	}

	server := httptest.NewServer(http.FileServer(fileSystem))
	defer server.Close()

	conn, err := cr.NewConn(ctx, server.URL+"/"+encodeDecodeHTML)
	if err != nil {
		return errors.Wrap(err, "failed to open webcodecs page")
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "timed out waiting for page loading")
	}

	var codec string = ""
	switch args.codec {
	case videotype.H264:
		codec = "avc1.42001E"
	case videotype.VP8:
		codec = "vp8"
	case videotype.VP9:
		codec = "vp09.00.10.08"
	}

	if err := conn.Call(ctx, nil, "EncodeDecode", codec, args.encoder, args.decoder, args.source); err != nil {
		return errors.Wrap(err, "error executing EncodeDecode()")
	}

	if err := conn.WaitForExpr(ctx, "TEST.complete()"); err != nil {
		return errors.Wrap(err, "error completing EncodeDecode()")
	}

	var success bool
	if err := conn.Eval(ctx, "TEST.success()", &success); err != nil {
		return errors.Wrap(err, "error getting TEST.success")
	}
	if !success {
		var logs string
		if err := conn.Eval(ctx, "TEST.getLogs()", &logs); err != nil {
			return errors.Wrap(err, "error getting TEST.logs")
		}

		return errors.Errorf("failed WebCodecs encoding and decoding: %v", logs)
	}

	return nil
}
