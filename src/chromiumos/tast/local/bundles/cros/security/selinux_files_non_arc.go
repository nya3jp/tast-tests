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
		Func:         SELinuxFilesNonARC,
		Desc:         "Checks SELinux labels on Chrome-specific files on devices that don't support ARC",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "selinux", "no_android"},
	})
}

func SELinuxFilesNonARC(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.NoLogin())
	defer cr.Close(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	for _, testArg := range []struct {
		path      string // absolute file path
		context   string // expected SELinux file context
		recursive bool
		filter    selinux.FileLabelCheckFilter
	}{
		{"/opt/google/chrome/chrome", "chrome_browser_exec", false, nil},
		{"/run/chrome/wayland-0", "wayland_socket", false, nil},
	} {
		filter := testArg.filter
		if filter == nil {
			filter = selinux.CheckAll
		}
		expected, err := selinux.FileContextRegexp(testArg.context)
		if err != nil {
			s.Errorf("Failed to compile expected context %q: %v", testArg.context, err)
			continue
		}
		selinux.CheckContext(s, testArg.path, expected, testArg.recursive, filter)
	}
}
