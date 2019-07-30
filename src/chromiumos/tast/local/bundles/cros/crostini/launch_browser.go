// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchBrowser,
		Desc:         "Opens a browser window on the host from the container, using several common approahces (/etc/alternatives, $BROWSER, and xdg-open)",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func LaunchBrowser(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	cont := pre.Container

	checkLaunch := func(urlTarget string, command ...string) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		cmd := cont.Command(ctx, command...)
		s.Logf("Running: %q", strings.Join(cmd.Args, " "))
		if err := cmd.Run(); err != nil {
			s.Error("Failed to launch browser from container: ", err)
			cmd.DumpLog(ctx)
			return
		}

		s.Logf("Waiting for renderer with URL %q", urlTarget)
		conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
			return t.URL == urlTarget
		})
		if err != nil {
			s.Error("Didn't see crosh renderer: ", err)
		} else {
			conn.CloseTarget(ctx)
			conn.Close()
		}
	}

	checkLaunch("http://x-www-browser.test/", "/etc/alternatives/x-www-browser", "http://x-www-browser.test/")
	checkLaunch("http://browser-env.test/", "sh", "-c", "${BROWSER} http://browser-env.test/")
	checkLaunch("http://xdg-open.test/", "xdg-open", "http://xdg-open.test/")
}
