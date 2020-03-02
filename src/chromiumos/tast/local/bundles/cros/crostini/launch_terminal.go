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
		Func:         LaunchTerminal,
		Desc:         "Executes the x-terminal-emulator alternative in the container which should then cause Chrome to open the Terminal extension",
		Contacts:     []string{"davidmunro@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraSoftwareDeps: []string{"crostini_stable"},
			},
			{
				Name:              "artifact_unstable",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraSoftwareDeps: []string{"crostini_unstable"},
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:      "download",
				Pre:       crostini.StartedByDownload(),
				Timeout:   10 * time.Minute,
				ExtraAttr: []string{"informational"},
			},
			{
				Name:      "download_buster",
				Pre:       crostini.StartedByDownloadBuster(),
				Timeout:   10 * time.Minute,
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func LaunchTerminal(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	cont := pre.Container

	const terminalURLContains = "://terminal/html/terminal.html?command=vmshell"

	checkLaunch := func(urlSuffix string, command ...string) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		cmd := cont.Command(ctx, command...)
		s.Logf("Running: %q", strings.Join(cmd.Args, " "))
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			s.Error("Failed to launch terminal command in container: ", err)
			return
		}

		s.Logf("Waiting for renderer with URL containing %q and suffix %q", terminalURLContains, urlSuffix)
		conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
			return strings.Contains(t.URL, terminalURLContains) &&
				strings.HasSuffix(t.URL, urlSuffix)
		})
		if err != nil {
			s.Error("Didn't see crosh renderer: ", err)
		} else {
			conn.CloseTarget(ctx)
			conn.Close()
		}
	}

	checkLaunch("", "x-terminal-emulator")

	// When we pass an argument to the x-terminal-emulator alternative, it should
	// then append that as URL parameters which will cause the terminal to
	// execute that command initially.
	checkLaunch("&args[]=--&args[]=vim", "x-terminal-emulator", "vim")
}
