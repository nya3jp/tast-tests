// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskmanager

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
)

// ProcessStatus defines the status of the process.
type ProcessStatus string

const (
	// ProcessAlive represents the status of the alive process.
	ProcessAlive ProcessStatus = "alive"
	// ProcessDead represents the status of the dead process.
	ProcessDead ProcessStatus = "dead"
	// ProcessUnknownStatus represents the unknown status of the process.
	ProcessUnknownStatus ProcessStatus = "unknown"
)

// Process defines the interface for the process.
type Process interface {
	// Open opens the process.
	Open(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error
	// Close closes the process.
	Close(ctx context.Context) error
	// Status returns the status of the process, e.g., alive, dead, and etc.
	Status(ctx context.Context, tconn *chrome.TestConn) (ProcessStatus, error)
	// NameInTaskManager returns the process name displayed in the task manager.
	NameInTaskManager() string
}

// ChromeTab defines the struct for chrome tab.
type ChromeTab struct {
	// URL is the url of the tab.
	URL string

	// ID is the id of the tab.
	ID int `json:"id"`
	// Title is the title of the tab.
	Title string `json:"title"`
	// LoadingStatus is the loading status of the tab.
	LoadingStatus TabStatus `json:"status"`

	// openInNewWindow indicates if the tab will be opened in a new window.
	// This field needs to be set before opening the tab to open the tab in a new window.
	openInNewWindow bool

	// conn is the connection to the chrome tab.
	conn *chrome.Conn
}

// NewChromeTabProcess returns an instance of ChromeTab.
func NewChromeTabProcess(url string) *ChromeTab {
	return &ChromeTab{
		URL: url,
	}
}

// SetOpenInNewWindow sets openInNewWindow to be true.
func (tab *ChromeTab) SetOpenInNewWindow() {
	tab.openInNewWindow = true
}

// Open opens a new chrome tab in a single browser window.
func (tab *ChromeTab) Open(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) (retErr error) {
	if tab.conn != nil {
		return errors.New("the tab is already opened")
	}

	var err error
	if tab.openInNewWindow {
		if tab.conn, err = cr.NewConn(ctx, tab.URL, browser.WithNewWindow()); err != nil {
			return errors.Wrapf(err, "failed to open %s", tab.URL)
		}
	} else {
		if err = kb.Accel(ctx, "Ctrl+T"); err != nil {
			return errors.Wrap(err, "failed to use Ctrl+T to open a new tab")
		}

		if tab.conn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/")); err != nil {
			return errors.Wrap(err, "failed to find the target")
		}

		if err := tab.conn.Navigate(ctx, tab.URL); err != nil {
			return errors.Wrapf(err, "failed to navigate to %s", tab.URL)
		}
	}

	defer func() {
		if retErr != nil {
			tab.Close(ctx)
		}
	}()

	if err := webutil.WaitForQuiescence(ctx, tab.conn, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait for web page to finish loading")
	}

	expr := `async () => {
	    const tabs = await tast.promisify(chrome.tabs.query)({currentWindow: true, active: true});
		if (tabs.length !== 1) {
			throw new Error("unexpected number of tabs: got " + tabs.length)
		}
	    return tabs[0]
	   }`

	if err := tconn.Call(ctx, &tab, expr); err != nil {
		return errors.Wrap(err, "failed to get current tab")
	}

	return nil
}

// Close closes the chrome tab.
func (tab *ChromeTab) Close(ctx context.Context) error {
	if tab.conn == nil {
		return errors.New("the tab is already closed")
	}

	if err := tab.conn.CloseTarget(ctx); err != nil {
		return errors.Wrapf(err, "failed to close the tab %q", tab.Title)
	}
	if err := tab.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close connection")
	}

	tab.conn = nil
	return nil
}

// TabStatus defines the 3 types of TabStatus in Chrome API.
type TabStatus string

// Define 3 types of TabStatus.
// See: https://developer.chrome.com/docs/extensions/reference/tabs/#type-TabStatus
const (
	TabUnloaded TabStatus = "unloaded"
	TabLoading  TabStatus = "loading"
	TabComplete TabStatus = "complete"
)

// Status returns the ProcessStatus of the chrome tab process.
func (tab *ChromeTab) Status(ctx context.Context, tconn *chrome.TestConn) (ProcessStatus, error) {
	tabStatus, err := tab.QueryLoadingStatus(ctx, tconn)
	if err != nil {
		return ProcessUnknownStatus, err
	}

	switch tabStatus {
	case TabLoading, TabComplete:
		return ProcessAlive, nil
	case TabUnloaded:
		return ProcessDead, nil
	default:
		return ProcessUnknownStatus, errors.New("unexpected status")
	}
}

// QueryLoadingStatus returns the TabStatus of the chrome tab.
func (tab *ChromeTab) QueryLoadingStatus(ctx context.Context, tconn *chrome.TestConn) (TabStatus, error) {
	expr := `async (id) => {
	    const tab = tast.promisify(chrome.tabs.get)(id);
	    return tab
	   }`
	if err := tconn.Call(ctx, &tab, expr, tab.ID); err != nil {
		return "", errors.Wrap(err, "failed to query tab")
	}

	return tab.LoadingStatus, nil
}

// NameInTaskManager returns the process name displayed in the task manager.
func (tab *ChromeTab) NameInTaskManager() string {
	return "Tab: " + tab.Title
}
