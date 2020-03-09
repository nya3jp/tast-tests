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
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Timeout:      arc.BootTimeout,
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func NetworkListenersARC(ctx context.Context, s *testing.State) {
	ls := netlisten.Common(s.PreValue().(arc.PreData).Chrome)
	ls["127.0.0.1:5037"] = "/usr/bin/adb"
	// patchpaneld runs an ADB proxy server on port 5555 whenever ARC is running. The proxy end listens only when ADB sideloading or ADB debugging on dev mode is enabled.
	ls["*:5555"] = "/usr/bin/patchpaneld"
	// sslh is installed on ARC-capable systems to multiplex port 22 traffic between sshd and patchpaneld (for adb).
	ls["*:22"] = "/usr/sbin/sslh-fork"
	ls["*:2222"] = "/usr/sbin/sshd"
	netlisten.CheckPorts(ctx, s, ls)
}
