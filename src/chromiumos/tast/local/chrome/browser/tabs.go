// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browser implements a layer of abstraction over Ash and Lacros Chrome
// instances.
package browser

import (
	"context"
	"fmt"
	"strconv"

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
	if err := tconn.Eval(ctx, "tast.promisify(chrome.tabs.query)({})", &tabs); err != nil {
		return nil, err
	}
	return tabs, nil
}

// CloseTabsByID closes the browser tabs given by their IDs.
func CloseTabsByID(ctx context.Context, tconn *TestConn, tabIDs []int) error {
	str := "["
	for i, id := range tabIDs {
		str += strconv.Itoa(id)
		if i != len(tabIDs)-1 {
			str += ", "
		}
	}
	str += "]"
	return tconn.Eval(ctx, fmt.Sprintf("tast.promisify(chrome.tabs.remove)(%s)", str), nil)
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

// ReplaceCurrentTabsWithSingleNewTab replaces the browser tabs of the current window
// with an empty tab.
// Leaving one tab is critical to keep the lacros-chrome process running.
// See crbug.com/1268743 for the chrome arg --disable-lacros-keep-alive.
// TODO(neis): Try to get rid of this function.
func ReplaceCurrentTabsWithSingleNewTab(ctx context.Context, tconn *TestConn) error {
	tabs, err := CurrentTabs(ctx, tconn)
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
