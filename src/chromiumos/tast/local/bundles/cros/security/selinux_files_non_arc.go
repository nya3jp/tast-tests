// Copyright 2018 The ChromiumOS Authors
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
		Func: SELinuxFilesNonARC,
		// TODO(b/254328916): Make sure we also cover the rootfs chrome binary in lacros variant tests.
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Checks SELinux labels on Chrome-specific files on devices that don't support ARC",
		Contacts:     []string{"fqj@chromium.org", "jorgelo@chromium.org", "chromeos-security@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "selinux", "no_android"},
	})
}

func SELinuxFilesNonARC(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
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
		{path: "/opt/google/chrome/chrome", context: "chrome_browser_exec"},
		{path: "/run/chrome/wayland-0", context: "wayland_socket"},
		{path: "/run/session_manager", context: "cros_run_session_manager", recursive: true},
		{path: "/var/log/chrome", context: "cros_var_log_chrome", recursive: true},
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
		selinux.CheckContext(ctx, s, &selinux.CheckContextReq{
			Path:         testArg.path,
			Expected:     expected,
			Recursive:    testArg.recursive,
			Filter:       filter,
			IgnoreErrors: false,
			Log:          false,
		})
	}
	selinux.CheckHomeDirectory(ctx, s)
}
