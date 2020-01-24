// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ChromeTask implements MemoryTask to open Chrome tabs.
type ChromeTask struct {
	// URLs is a cyclical list of URLs to be opened.
	URLs []string
	// NumTabs is the total number of tabs to be opened.
	NumTabs int
	// conns is a list of connections to Chrome renderers
	conns []*chrome.Conn
}

// Run opens the number of tabs defined in ChromeTask, cycling through the list of URLs
// defined in ChromeTask for each new tab.
func (ct *ChromeTask) Run(ctx context.Context, s *testing.State, testEnv *TestEnv) error {
	for i := 0; i < ct.NumTabs; i++ {
		url := ct.URLs[i%len(ct.URLs)]
		conn, err := testEnv.cr.NewConn(ctx, url)
		if err != nil {
			return errors.Wrapf(err, "failed to open %s in tab %d", url, i)
		}
		ct.conns = append(ct.conns, conn)
		if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
			return errors.Wrap(err, "waiting for load failed")
		}
	}
	return nil
}

// Close closes all of the Chrome connections defined in ChromeTask, created in Run.
func (ct *ChromeTask) Close(ctx context.Context, testEnv *TestEnv) {
	for _, conn := range ct.conns {
		conn.Close()
	}
}

// String returns a string describing the ChromeTask.
func (ct *ChromeTask) String() string {
	return fmt.Sprintf("ChromeTask with URLs: %s, NumTabs: %d", ct.URLs, ct.NumTabs)
}

// NeedVM returns false to indicate that no VM is required for a ChromeTask.
func (ct *ChromeTask) NeedVM() bool {
	return false
}
