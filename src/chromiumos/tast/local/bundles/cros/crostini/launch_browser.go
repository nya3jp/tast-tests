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
		Desc:         "Opens a browser window on the host from the container, using several common approaches (/etc/alternatives, $BROWSER, and xdg-open)",
		Contacts:     []string{"davidmunro@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"keepState"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniStable,
			},
			{
				Name:              "artifact_unstable",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
			},
			{
				Name:    "download_stretch",
				Pre:     crostini.StartedByDownloadStretch(),
				Timeout: 10 * time.Minute,
			},
			{
				Name:    "download_buster",
				Pre:     crostini.StartedByDownloadBuster(),
				Timeout: 10 * time.Minute,
			},
		},
	})
}

func LaunchBrowser(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	cont := pre.Container
	defer crostini.RunCrostiniPostTest(ctx,
		s.PreValue().(crostini.PreData).Container,
		s.PreValue().(crostini.PreData).Chrome.User())

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
		conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(urlTarget))
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
