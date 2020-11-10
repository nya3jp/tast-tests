// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Chrome,
		SoftwareDeps: []string{"chrome"},
	})
}

func Chrome(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.RecordScreen(filepath.Join(s.OutDir(), "screen.mp4")))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.NewConn(ctx, "https://en.wikipedia.org/wiki/Chromium_OS")
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer conn.Close()

	testing.Sleep(ctx, 5*time.Second)
}
