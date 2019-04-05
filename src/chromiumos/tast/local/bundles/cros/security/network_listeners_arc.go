// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/security/netlisten"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NetworkListenersARC,
		Desc: "Checks TCP listeners on ARC systems",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome_login", "android"},
		Pre:          arc.Booted(),
		Timeout:      arc.BootTimeout,
	})
}

func NetworkListenersARC(ctx context.Context, s *testing.State) {
	ls := netlisten.Common(s.PreValue().(arc.PreData).Chrome)
	ls["127.0.0.1:5037"] = "/usr/bin/adb"
	// sslh is installed on ARC-capable systems to multiplex port 22 traffic between sshd and adb.
	ls["*:22"] = "/usr/sbin/sslh-fork"
	ls["*:2222"] = "/usr/sbin/sshd"
	netlisten.CheckPorts(ctx, s, ls)
}
