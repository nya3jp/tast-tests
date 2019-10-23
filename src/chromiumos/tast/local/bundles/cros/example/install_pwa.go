// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InstallPWA,
		Desc:         "Demonstrates how to use the chrome.display API",
		Contacts:     []string{"tast-owners@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func InstallPWA(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}

	if _, err := cr.NewConn(ctx, "https://squoosh.app"); err != nil {
		s.Fatal("Could not open connection: ", err)
	}

	if err := installPWAForCurrentURL(ctx, tconn); err != nil {
		s.Fatal("Failed to install PWA: ", err)
	}
}

func installPWAForCurrentURL(ctx context.Context, tconn *chrome.Conn) error {
	var appID string
	err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.installPWAForCurrentURL)(5000)`, &appID)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "install appId: %q", appID)
	return nil
}
