// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontlineworkercuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// defaultUIWaitTime indicates the default time to wait for UI elements to appear.
	defaultUIWaitTime = 5 * time.Second
	// longerUIWaitTime indicates the time to wait for some UI elements that need more time to appear.
	longerUIWaitTime = time.Minute
)

// Workload indicates the workload of the case.
type Workload uint8

// Browsering is for testing the browsering workload.
// Collaborating is for testing the collaborating workload.
const (
	Browsering Workload = iota
	Collaborating
)

// OpenGoogleTabs opens the specified number of chrome tabs with Google URL.
func OpenGoogleTabs(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, uiHdl cuj.UIActionHandler, numberOfTabs int) error {
	ui := uiauto.New(tconn)
	link := nodewith.Name("English").Role(role.Link)
	for i := 0; i < numberOfTabs; i++ {
		_, err := cr.NewConn(ctx, cuj.GoogleURL)
		if err != nil {
			return errors.Wrapf(err, "the current tab index: %d, failed to open URL: %s", i, cuj.GoogleURL)
		}
		// Since visitors come from different countries, the default language of Google's website is different.
		// Change language to "English" if the English language link exists.
		if err := ui.IfSuccessThen(ui.WithTimeout(defaultUIWaitTime).WaitUntilExists(link), uiHdl.Click(link))(ctx); err != nil {
			return err
		}
	}
	return nil
}

// EnterSearchTerms enters search terms on each tab.
func EnterSearchTerms(ctx context.Context, uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter, searchTerms []string) error {
	for i := 0; i < len(searchTerms); i++ {
		if i == 0 {
			// The first tab will be an empty tab. Therefore, the operation needs to start from the second tab.
			if err := uiHdl.SwitchToNextChromeTab()(ctx); err != nil {
				return err
			}
		}
		testing.ContextLog(ctx, "Switching to chrome tab index: ", i+1)
		if err := uiauto.Combine("enter search term",
			uiHdl.SwitchToNextChromeTab(),
			kb.TypeAction(string(searchTerms[i])),
			kb.AccelAction("Enter"),
		)(ctx); err != nil {
			return err
		}
	}
	return nil
}

// ScrollTabPage scrolls the specified tab index of the webpage.
func ScrollTabPage(ctx context.Context, uiHdl cuj.UIActionHandler, idx int) error {
	scrollActions := uiHdl.ScrollChromePage(ctx)
	if err := uiHdl.SwitchToChromeTabByIndex(idx)(ctx); err != nil {
		return errors.Wrap(err, "failed to switch tab")
	}
	for _, act := range scrollActions {
		if err := act(ctx); err != nil {
			return errors.Wrap(err, "failed to execute action")
		}
	}
	return nil
}
