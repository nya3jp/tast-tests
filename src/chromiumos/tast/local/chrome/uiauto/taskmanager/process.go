// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskmanager

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

// Process defines the interface for the process.
type Process interface {
	Open(ctx context.Context, cr *chrome.Chrome) error
	Close(ctx context.Context)
	NameInTaskManager() string
}

// ChromeTab defines the struct for chrome tab.
type ChromeTab struct {
	URL  string
	Name string
	conn *chrome.Conn
}

// NewChromeTabProcess returns an instance of ChromeTab.
func NewChromeTabProcess(url, name string) *ChromeTab {
	return &ChromeTab{
		URL:  url,
		Name: name,
	}
}

// Open opens a new chrome tab.
func (tab *ChromeTab) Open(ctx context.Context, cr *chrome.Chrome) (err error) {
	if tab.conn, err = cr.NewConn(ctx, tab.URL); err != nil {
		return errors.Wrapf(err, "failed to open %s", tab.URL)
	}
	defer func() {
		if err != nil {
			tab.Close(ctx)
		}
	}()

	if err := webutil.WaitForQuiescence(ctx, tab.conn, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait for web page to finish loading")
	}

	return nil
}

// Close closes the chrome tab.
func (tab *ChromeTab) Close(ctx context.Context) {
	if tab.conn == nil {
		return
	}

	if err := tab.conn.CloseTarget(ctx); err != nil {
		testing.ContextLogf(ctx, "Failed to close the tab %q: %v", tab.Name, err)
	}
	if err := tab.conn.Close(); err != nil {
		testing.ContextLog(ctx, "Failed to close connection: ", err)
	}

	tab.conn = nil
}

// NameInTaskManager returns the display name of a process in the task manager.
func (tab *ChromeTab) NameInTaskManager() string {
	return "Tab: " + tab.Name
}
