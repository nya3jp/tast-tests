// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
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

func SELinuxFileLabelWithChrome(s *testing.State) {
	ctx := s.Context()
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
		{"/run/chrome/wayland-0", "u:object_r:wayland_socket:s0", false, nil},
		{"/opt/google/chrome/chrome", "u:object_r:chrome_browser_exec:s0", false, nil},
	} {
		filter := testArg.filter
		if filter == nil {
			filter = selinux.CheckAll
		}
		selinux.CheckContext(s, testArg.path, testArg.context, testArg.recursive, filter)
	}
}
