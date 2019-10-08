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
	"io/ioutil"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

const (
	// ChromeMediaInternalsUtilsJSFile is a JS file containing utils to interact with chrome://media-internals.
	ChromeMediaInternalsUtilsJSFile = "chrome_media_internals_utils.js"
)

// OpenChromeMediaInternalsPageAndInjectJS opens a chrome://media-internals tab
// in cr and injects into it the optional JS code in JSPath.
func OpenChromeMediaInternalsPageAndInjectJS(ctx context.Context, cr *chrome.Chrome, JSPath string) (conn *chrome.Conn, retErr error) {
	extraJS, err := ioutil.ReadFile(JSPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read JS file %s", JSPath)
	}

	const url = "chrome://media-internals"
	conn, err = cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s", url)
	}
	defer func() {
		if retErr != nil {
			conn.Close()
		}
	}()

	err = conn.WaitForExpr(ctx, "document.readyState === 'complete'")
	if err != nil {
		return nil, err
	}
	if err = conn.Exec(ctx, string(extraJS)); err != nil {
		return nil, err
	}
	return conn, nil
}

// URLUsesPlatformVideoDecoder digs into chrome://media-internals to find
// out if the given url is/was played with a platform video decoder (i.e. a
// hardware accelerated video decoder).
// chromeMediaInternalsConn must be a connection to a media-internals tab.
func URLUsesPlatformVideoDecoder(ctx context.Context, chromeMediaInternalsConn *chrome.Conn, url string) (uses bool, err error) {
	code := fmt.Sprintf(`checkChromeMediaInternalsIsPlatformVideoDecoderForURL(%q);`, url)
	if err := chromeMediaInternalsConn.EvalPromise(ctx, code, &uses); err != nil {
		return false, errors.Wrap(err, "failed to read chrome://media-internals JS")
	}
	return uses, err
}
