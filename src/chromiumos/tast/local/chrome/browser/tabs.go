// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browser implements a layer of abstraction over Ash and Lacros Chrome
// instances.
package browser

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
)

// Tab represents a browser tab as obtained from the chrome.tabs API.
// See https://developer.chrome.com/docs/extensions/reference/tabs/#type-Tab
type Tab struct {
	ID     int    `json:"ID"`
	Index  int    `json:"index"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Active bool   `json:"active"`
}

// CurrentTabs returns the tabs of the current browser window.
// The browser is given via |tconn|.
func CurrentTabs(ctx context.Context, tconn *TestConn) ([]Tab, error) {
	var tabs []Tab
	if err := tconn.Eval(ctx, "tast.promisify(chrome.tabs.query)({currentWindow: true})", &tabs); err != nil {
		return nil, err
	}
	return tabs, nil
}

// AllTabs returns the tabs of all browser windows.
// The browser is given via |tconn|.
func AllTabs(ctx context.Context, tconn *TestConn) ([]Tab, error) {
	var tabs []Tab
	if err := tconn.Eval(ctx, `(async () => {
		const tabs = await tast.promisify(chrome.tabs.query)({});
		return tabs.filter((tab) => tab.id);
	})()`, &tabs); err != nil {
		return nil, err
	}
	return tabs, nil
}

// CloseTabsByID closes the browser tabs given by their IDs.
func CloseTabsByID(ctx context.Context, tconn *TestConn, tabIDs []int) error {
	return tconn.Call(ctx, nil, "tast.promisify(chrome.tabs.remove)", tabIDs)
}

// CloseAllTabs closes all browser tabs and therefore all browser windows.
func CloseAllTabs(ctx context.Context, tconn *TestConn) error {
	return tconn.Eval(ctx, `(async () => {
		const tabs = await tast.promisify(chrome.tabs.query)({});
		await tast.promisify(chrome.tabs.remove)(tabs.filter((tab) => tab.id).map((tab) => tab.id));
	})()`, nil)
}

// TODO(neis): Put this in a common place.
const newTabURL = "chrome://new-tab/"

// ReplaceAllTabsWithSingleNewTab replaces the browser tabs of the current window
// with an empty tab.
// Leaving one tab is critical to keep the lacros-chrome process running.
// See crbug.com/1268743 for the chrome arg --disable-lacros-keep-alive.
// TODO(neis): Try to get rid of this function.
func ReplaceAllTabsWithSingleNewTab(ctx context.Context, tconn *TestConn) error {
	tabs, err := AllTabs(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get browser tabs")
	}
	if len(tabs) == 0 {
		return errors.New("browser has no tabs")
	}
	if len(tabs) == 1 && tabs[0].URL == newTabURL {
		return nil
	}
	// Simply create a new tab and close all the others.
	if err := tconn.Eval(ctx, "tast.promisify(chrome.tabs.create)({})", nil); err != nil {
		return errors.Wrap(err, "failed to create new tab")
	}
	var tabsToClose []int
	for _, t := range tabs {
		tabsToClose = append(tabsToClose, t.ID)
	}
	if err := CloseTabsByID(ctx, tconn, tabsToClose); err != nil {
		return errors.Wrap(err, "failed to close other browser tabs")
	}
	return nil
}

// GetTabByTitle gets a single tab that has a tab title that
// matches |title|. It returns an error if there is not exactly 1 tab
// that meets the criterion. The browser is given via |tconn|.
func GetTabByTitle(ctx context.Context, tconn *TestConn, title string) (*Tab, error) {
	var tabs []Tab
	if err := tconn.Eval(ctx, fmt.Sprintf(`(async () => {
		return await tast.promisify(chrome.tabs.query)({title: %q})
	})()`, title), &tabs); err != nil {
		return nil, errors.Wrapf(err, "failed to search for tabs with title %q", title)
	}

	if len(tabs) != 1 {
		return nil, errors.Errorf("unexpected number of tabs with title %q; found %d, expected 1", title, len(tabs))
	}
	return &tabs[0], nil
}

// CloseTabByTitle finds a tab with title |title| and closes
// that tab. If there are multiple tabs with that title, it returns
// an error. The browser is given via |tconn|.
func CloseTabByTitle(ctx context.Context, tconn *TestConn, title string) error {
	tab, err := GetTabByTitle(ctx, tconn, title)
	if err != nil {
		return err
	}
	return CloseTabsByID(ctx, tconn, []int{tab.ID})
}
