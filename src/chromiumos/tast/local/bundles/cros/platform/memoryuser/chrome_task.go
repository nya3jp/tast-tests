// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memoryuser contains common code to run multifaceted memory tests
// with Chrome, ARC, and VMs
package memoryuser

import (
	"context"

	"chromiumos/tast/testing"
)

// ChromeTask contains a list of Urls that can be opened and NumTabs, which is the total number of
// tabs to be opened.
type ChromeTask struct {
	Urls    []string
	NumTabs int
}

// RunMemoryTask opens the number of tabs defined in ChromeTask, cycling through the list of urls
// defined in ChromeTask for each new tab.
func (chromeTask ChromeTask) RunMemoryTask(ctx context.Context, s *testing.State, testEnv *TestEnvironment) {
	for i := 0; i < chromeTask.NumTabs; i++ {
		index := i % len(chromeTask.Urls)
		conn, err := testEnv.Chrome.NewConn(ctx, chromeTask.Urls[index])
		if err != nil {
			s.Fatal("Failed to open page: ", err)
		}
		if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
			s.Fatal("Waiting load failed: ", err)
		}
	}

}
