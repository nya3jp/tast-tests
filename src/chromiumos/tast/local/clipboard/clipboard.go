// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package clipboard contains utilities to manage the clipboard.
package clipboard

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// GetClipboardItemsSize returns the number of items currently in the clipboard.
func GetClipboardItemsSize(ctx context.Context, tconn *chrome.TestConn) (int, error) {
	var result int
	if err := tconn.Call(ctx, &result, `
		  () => {
		    let result;
		    document.addEventListener('paste', (event) => {
		      result = event.clipboardData.items.length;
		    }, {once: true});
		    if (!document.execCommand('paste')) {
			    throw new Error('Failed to execute paste');
		    }
		    return result;
		  }`,
	); err != nil {
		return 0, errors.Wrap(err, "failed to get clipboard items size")
	}
	return result, nil
}

// GetClipboardFirstItemType returns the type of the first item in the clipboard.
// More information about the item can be found here
// https://developer.mozilla.org/en-US/docs/Web/API/DataTransferItem.
func GetClipboardFirstItemType(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var t string
	if err := tconn.Call(ctx, &t, `
		  () => {
		    let result;
		    document.addEventListener('paste', (event) => {
			  let items = event.clipboardData.items;
			  if (items.length == 0) {
				  throw new Error('Clipboard is empty');
			  }
		      result = items[0].type;
		    }, {once: true});
		    if (!document.execCommand('paste')) {
			    throw new Error('Failed to execute paste');
		    }
		    return result;
		  }`,
	); err != nil {
		return "", errors.Wrap(err, "failed to get clipboard item's type")
	}
	return t, nil
}
