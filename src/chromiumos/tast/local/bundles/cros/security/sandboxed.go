// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Sandboxed,
		Desc:         "Verify sandbox status",
		SoftwareDeps: []string{"chrome_login"},
	})
}

func Sandboxed(ctx context.Context, s *testing.State) {
	const (
		url      = "chrome://sandbox"
		text     = "You are adequately sandboxed."
		waitExpr = "document.getElementsByTagName('p')[0].textContent"
	)

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to create a new connection: ", err)
	}
	defer conn.Close()

	{
		ectx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err = conn.WaitForExpr(ectx, waitExpr); err != nil {
			s.Fatalf("Failed to evaluate in %q in %s", waitExpr, url)
		}
	}

	c, err := conn.PageContent(ctx)
	if err != nil {
		s.Fatal("Failed to obtain the page content")
	}

	if !strings.Contains(c, text) {
		s.Errorf("Could not find %q in %q in %s", text, c, url)
	}
}
