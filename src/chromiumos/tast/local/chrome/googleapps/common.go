// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package googleapps

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const (
	longUITimeout      = time.Minute     // Used for situations where UI elements that need more time to appear.
	shortUITimeout     = 5 * time.Second // Used for situations where UI response are faster.
	saveToDriveTimeout = 5 * time.Second // saveToDriveTimeout indicates the maximum waiting time for Google to save to Drive.
	retryTimes         = 3               // Used for some operations that need to be retried.
)

// waitForDocumentSaved waits for the document state to "Saved to Drive".
func waitForDocumentSaved(tconn *chrome.TestConn, appName string) action.Action {
	ui := uiauto.New(tconn)
	webArea := nodewith.NameContaining(appName).Role(role.RootWebArea)
	documentSavedState := nodewith.NameContaining("Document status: Saved to Drive").Role(role.Button).Ancestor(webArea)
	return func(ctx context.Context) error {
		startTime := time.Now()
		if err := ui.WithTimeout(saveToDriveTimeout).WaitUntilExists(documentSavedState)(ctx); err != nil {
			unableToLoadDialog := nodewith.Name("Unable to load file").Role(role.Dialog)
			if loadFileErr := ui.Exists(unableToLoadDialog)(ctx); loadFileErr == nil {
				return errors.New("unable to load file")
			}
			testing.ContextLog(ctx, "Failed to wait for document saved within ", saveToDriveTimeout)
		} else {
			testing.ContextLog(ctx, "Saved to drive in ", time.Now().Sub(startTime))
		}
		return nil
	}
}

func waitForFieldTextToBe(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string) action.Action {
	ui := uiauto.New(tconn)
	return ui.WithInterval(400*time.Millisecond).RetrySilently(5,
		func(ctx context.Context) error {
			nodeInfo, err := ui.Info(ctx, finder)
			if err != nil {
				return err
			}
			if nodeInfo.Value != expectedText {
				return errors.Errorf("failed to validate input value: got: %s; want: %s", nodeInfo.Value, expectedText)
			}
			return nil
		})
}
