// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sched

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RendererCoreSched,
		Desc:         "Ensures renderers are assigned different scheduling cookies",
		Contacts:     []string{"joelaf@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      3 * time.Minute,
		Pre:          chrome.LoggedIn(),
	})
}

func RendererCoreSched(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	newsConn, err := cr.NewConn(ctx, "https://news.google.com/")
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer newsConn.Close()
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := webutil.WaitForQuiescence(ctx, newsConn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	gmailConn, err := cr.NewConn(ctx, "https://gmail.com/")
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer gmailConn.Close()

	if err := webutil.WaitForQuiescence(ctx, gmailConn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}
}
