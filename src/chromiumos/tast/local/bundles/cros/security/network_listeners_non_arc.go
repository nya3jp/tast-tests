// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/bundles/cros/security/netlisten"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/moblab"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NetworkListenersNonARC,
		Desc: "Checks TCP listeners on non-ARC systems",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome", "no_android"},
		Pre:          chrome.LoggedIn(),
	})
}

func NetworkListenersNonARC(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	ls, err := netlisten.Common(cr)
	if err != nil {
		s.Fatal("Failed to find common network listeners: ", err)
	}
	ls["*:22"] = "/usr/sbin/sshd"

	if moblab.IsMoblab() {
		ls["*:80"] = "/usr/sbin/apache2"
		ls["127.0.0.1:3306"] = "/usr/sbin/mysqld"
		ls["*:8080"] = "/usr/bin/python2.7"
		ls["*:9991"] = "/usr/bin/python2.7"
	}

	netlisten.CheckPorts(ctx, s, ls)
}
