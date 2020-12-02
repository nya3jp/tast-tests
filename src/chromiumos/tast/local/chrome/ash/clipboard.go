// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// ClipboardTextData returns clipboard text data.
func ClipboardTextData(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var data string

	if err := tconn.Call(ctx, &data, `tast.proimsify(chrome.autotestPrivate.getClipboardTextData)`); err != nil {
		return "", err
	}

	return data, nil
}

// SetClipboard forcibly sets the clipboard to the given data.
func SetClipboard(ctx context.Context, tconn *chrome.TestConn, data string) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setClipboardTextData)`, data)
}
