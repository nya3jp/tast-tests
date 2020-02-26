// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package settings implements a library used for communication with Chrome settings.
// A chrome.TestConn returned by TestAPIConn() with the "settingsPrivate" permission is needed.
package settings

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// DefaultZoom returns the default page zoom factor. Possible values are currently between
// 0.25 and 5. For a full list, see zoom::kPresetZoomFactors in:
// https://cs.chromium.org/chromium/src/components/zoom/page_zoom_constants.cc
func DefaultZoom(ctx context.Context, tconn *chrome.TestConn) (float64, error) {
	var zoom float64
	if err := tconn.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.settingsPrivate.getDefaultZoom(function(zoom) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    resolve(zoom);
		  })
		})`, &zoom); err != nil {
		return 0, err
	}
	return zoom, nil
}

// SetDefaultZoom sets the page zoom factor. Must be less than 0.001 different than a value
// in zoom::kPresetZoomFactors. See:
// https://cs.chromium.org/chromium/src/components/zoom/page_zoom_constants.cc
func SetDefaultZoom(ctx context.Context, tconn *chrome.TestConn, zoom float64) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.settingsPrivate.setDefaultZoom(%f, function(success) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    if (!success) {
		      reject(new Error("setDefaultZoom() failed"));
		      return;
		    }
		    resolve();
		  })
		})`, zoom)
	return tconn.EvalPromise(ctx, expr, nil)
}
