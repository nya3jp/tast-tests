// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// GetPastedData returns clipboard content.
func GetPastedData(tconn *chrome.TestConn, format string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		var result string
		if err := tconn.Call(ctx, &result, `
		  (format) => {
		    let result;
		    document.addEventListener('paste', (event) => {
		      result = event.clipboardData.getData(format);
		    }, {once: true});
		    if (!document.execCommand('paste')) {
			    throw new Error('Failed to execute paste');
		    }
		    return result;
		  }`, format,
		); err != nil {
			return "", err
		}
		return result, nil
	}
}

// CheckPasteNode checks if paste node exists.
func CheckPasteNode(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	pasteNode := nodewith.Name("Paste Ctrl+V").Role(role.MenuItem)

	if err := uiauto.Combine("Check paste node greyed",
		ui.WaitUntilExists(pasteNode))(ctx); err != nil {
		return errors.Wrap(err, "failed to check paste node greyed: ")
	}

	return nil
}
