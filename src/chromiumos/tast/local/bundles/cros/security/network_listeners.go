// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/bundles/cros/security/netlisten"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NetworkListeners,
		Desc:         "Checks TCP listeners while logged in without ARC",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func NetworkListeners(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in: ", err)
	}
	defer cr.Close(ctx)

	expected := map[string]string{}
	for addrPort, exe := range netlisten.SSHListeners(ctx) {
		expected[addrPort] = exe
	}
	for addrPort, exe := range netlisten.ChromeListeners(ctx, cr) {
		expected[addrPort] = exe
	}
	netlisten.CheckPorts(ctx, s, expected)
}
