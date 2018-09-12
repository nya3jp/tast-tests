// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// LaunchBrowser executes the x-www-browser alternative, uses the $BROWSER env
// variable and also runs xdg-open in the container with test URLs which should
// then cause Chrome to open a browser tab at the target address.
func LaunchBrowser(s *testing.State, cr *chrome.Chrome, cont *vm.Container) {
	s.Log("Executing LaunchBrowser test")

	checkLaunch := func(urlTarget string, command ...string) {
		ctx, cancel := context.WithTimeout(s.Context(), 10*time.Second)
		defer cancel()

		cmd := cont.Command(s.Context(), command...)
		s.Logf("Running: %q", strings.Join(cmd.Args, " "))
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(s.Context())
			s.Error("Failed to launch browser from container: ", err)
			return
		}

		s.Logf("Waiting for renderer with URL %q", urlTarget)
		conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
			return t.URL == urlTarget
		})
		if err != nil {
			s.Error("Didn't see crosh renderer: ", err)
		} else {
			conn.Close()
		}
	}

	checkLaunch("http://x-www-browser.test/", "/etc/alternatives/x-www-browser", "http://x-www-browser.test/")
	checkLaunch("http://browser-env.test/", "sh", "-c", "${BROWSER} http://browser-env.test/")
	checkLaunch("http://xdg-open.test/", "xdg-open", "http://xdg-open.test/")
}
