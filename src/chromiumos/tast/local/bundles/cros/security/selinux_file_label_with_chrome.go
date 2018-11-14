// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/bundles/cros/security/selinux"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFileLabelWithChrome,
		Desc:         "Checks that SELinux files managed by Chrome are set correctly",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "selinux"},
	})
}

func SELinuxFileLabelWithChrome(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.NoLogin())
	defer cr.Close(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	for _, testArg := range []struct {
		path, context string
		recursive     bool
		filter        selinux.FileLabelCheckFilter
	}{
		{"/opt/google/chrome/chrome", "chrome_browser_exec", false, nil},
		{"/run/chrome/wayland-0", "wayland_socket", false, nil},
	} {
		filter := testArg.filter
		if filter == nil {
			filter = selinux.CheckAll
		}
		selinux.CheckContext(s, testArg.path, selinux.S0Object(testArg.context), testArg.recursive, filter)
	}
}
