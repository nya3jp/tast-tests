// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webcodecs provides common code for video.WebCodecs* tests
package webcodecs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/mafredri/cdp/protocol/media"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

// HardwareAcceleration represents the preference of used codecs in WebCodecs API.
// See https://www.w3.org/TR/webcodecs/#hardware-acceleration.
type HardwareAcceleration string

const (
	// PreferHardware means hardware accelerated encoder/decoder is preferred.
	PreferHardware HardwareAcceleration = "prefer-hardware"
	// PreferSoftware means software encoder/decoder is preferred.
	PreferSoftware HardwareAcceleration = "prefer-software"
)

type videoConfig struct {
	width, height, numFrames, framerate int
}

// MP4DemuxerDataFiles returns the list of JS files for demuxing MP4 container.
func MP4DemuxerDataFiles() []string {
	return []string{
		"third_party/mp4/mp4_demuxer.js",
		"third_party/mp4/mp4box.all.min.js",
	}
}

// toMIMECodec converts videotype.Codec to codec in MIME type.
// See https://developer.mozilla.org/en-US/docs/Web/Media/Formats/codecs_parameter for detail.
func toMIMECodec(codec videotype.Codec) string {
	switch codec {
	case videotype.H264:
		// H.264 Baseline Level 3.1.
		return "avc1.42001F"
	case videotype.VP8:
		return "vp8"
	case videotype.VP9:
		// VP9 profile 0 level 1.0 8-bit depth.
		return "vp09.00.10.08"
	case videotype.AV1:
		// AV1 profile Main level 3.1 8-bit depth.
		// "level 3.1" means 1280x720@30fps, as per [1], is that intended,
		// i.e. to specify the resolution and framerate at configuration
		// time (and not just encode anything that is fed into the encoder).
		// [1] https://aomedia.org/av1/specification/annex-a/
		return "av01.0.05M.08"
	}
	return ""
}

// outputJSLogAndError outputs logs in JS and returns callErr. The part of outputting JS logs
// is common process before returning an test error.
func outputJSLogAndError(ctx context.Context, conn *chrome.Conn, callErr error) error {
	var logs string
	if err := conn.Eval(ctx, "TEST.getLogs()", &logs); err != nil {
		testing.ContextLog(ctx, "Error getting TEST.logs: ", err)
	}
	testing.ContextLog(ctx, "log=", logs)
	return callErr
}

func prepareWebCodecsTest(ctx context.Context, cs ash.ConnSource, fileSystem http.FileSystem, html string) (cleanupCtx context.Context, server *httptest.Server, conn *chrome.Conn, observer media.PlayerPropertiesChangedClient, deferFunc func(), err error) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		err = errors.Wrap(err, "failed to set values for verbose logging")
		return
	}
	defer func() {
		if err != nil {
			vl.Close()
		}
	}()

	cleanupCtx = ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	server = httptest.NewServer(http.FileServer(fileSystem))
	defer func() {
		if err != nil {
			server.Close()
		}
	}()
	testing.ContextLogf(ctx, "%s, %s", server.URL, html)
	conn, err = cs.NewConn(ctx, server.URL+"/"+html)
	if err != nil {
		err = errors.Wrap(err, "failed to open webcodecs page")
		return
	}
	defer func() {
		if err != nil {
			conn.Close()
			conn.CloseTarget(cleanupCtx)
		}
	}()

	if err = conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		err = errors.Wrap(err, "timed out waiting for page loading")
		return
	}
	observer, err = conn.GetMediaPropertiesChangedObserver(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to retrieve a media DevTools observer")
		return
	}

	deferFunc = func() {
		conn.Close()
		conn.CloseTarget(cleanupCtx)
		server.Close()
		cancel()
		vl.Close()
	}

	return
}
