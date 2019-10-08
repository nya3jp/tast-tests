// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file provides common code for interfacing with chrome://media-internals
// and querying information related to video playback, e.g. if it's using an
// accelerated decoder.

package decode

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// ChromeMediaInternalsUtilsJSFile is a JS file containing utils to interact with chrome://media-internals.
	ChromeMediaInternalsUtilsJSFile = "chrome_media_internals_utils.js"
)

// OpenChromeMediaInternalsPageAndInjectJS opens a chrome://media-internals tab
// in cr and injects into in the optional JS code in extraJS.
func OpenChromeMediaInternalsPageAndInjectJS(ctx context.Context, cr *chrome.Chrome, extraJS string) (*chrome.Conn, error) {
	const url = "chrome://media-internals"
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open "+url)
	}
	err = conn.WaitForExpr(ctx, "document.readyState === 'complete'")
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err = conn.Exec(ctx, extraJS); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// CheckIfURLUsesPlatformVideoDecoder digs into chrome://media-internals to find
// out if the given theURL is/was played with a platform video decoder (i.e. a
// hardware accelerated video decoder).
// chromeMediaInternalsConn must be a connection to a media-internals tab.
func CheckIfURLUsesPlatformVideoDecoder(ctx context.Context, s *testing.State, chromeMediaInternalsConn *chrome.Conn, theURL string) (usesPlatformVideoDecoder bool, err error) {
	checkForPlatformVideoDecoder :=
		fmt.Sprintf(`checkChromeMediaInternalsIsPlatformVideoDecoderForURL(%q);`, theURL)

	if err := chromeMediaInternalsConn.EvalPromise(ctx, checkForPlatformVideoDecoder, &usesPlatformVideoDecoder); err != nil {
		s.Fatal("Checking chrome://media-internals failed: ", err)
	}
	if err != nil {
		return false, err
	}
	return usesPlatformVideoDecoder, err
}
