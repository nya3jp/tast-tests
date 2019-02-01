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
		Func: NetworkListenersNonARC,
		Desc: "Checks TCP listeners on non-ARC systems",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome_login", "no_android"},
	})
}

func NetworkListenersNonARC(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in: ", err)
	}
	defer cr.Close(ctx)

	netlisten.CheckPorts(ctx, s, map[string]string{
		cr.DebugAddrPort(): chrome.ExecPath,
		"*:22":             "/usr/sbin/sshd",
		// Tast may forward port 28082 to the ephemeral devserver.
		"127.0.0.1:28082": "/usr/sbin/sshd",
	})
}
