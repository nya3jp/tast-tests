// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestExt,
		Desc:         "Test that shows demonstrates the test extension API works with logged in user",
		Contacts:     []string{"billyzhao@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
	})
}

func TestExt(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	var enabled struct {
		Value bool `json:"value"`
	}
	if err := tconn.Call(ctx, &enabled, "tast.promisify(chrome.settingsPrivate.getPref)", "ash.user.bluetooth.adapter_enabled"); err != nil {
		s.Fatal("Failed to get Pref: ", err)
	}

}
