// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchBrowser,
		Desc:         "Opens a browser window on the host from the container, using several common approaches (/etc/alternatives, $BROWSER, and xdg-open)",
		Contacts:     []string{"davidmunro@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Vars:         []string{"keepState"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			// Parameters generated by params_test.go. DO NOT EDIT.
			{
				Name:              "stretch_stable",
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("stretch", false), crostini.GetContainerRootfsArtifact("stretch", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Pre:               crostini.StartedByDlcStretch(),
				Timeout:           7 * time.Minute,
			}, {
				Name:              "stretch_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("stretch", false), crostini.GetContainerRootfsArtifact("stretch", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Pre:               crostini.StartedByDlcStretch(),
				Timeout:           7 * time.Minute,
			}, {
				Name:              "buster_stable",
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniStable,
				Pre:               crostini.StartedByDlcBuster(),
				Timeout:           7 * time.Minute,
			}, {
				Name:              "buster_unstable",
				ExtraAttr:         []string{"informational"},
				ExtraData:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
				ExtraSoftwareDeps: []string{"dlc"},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				Pre:               crostini.StartedByDlcBuster(),
				Timeout:           7 * time.Minute,
			},
		},
	})
}

func LaunchBrowser(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	cont := pre.Container
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	checkLaunch := func(urlTarget string, command ...string) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		cmd := cont.Command(ctx, command...)
		s.Logf("Running: %q", strings.Join(cmd.Args, " "))
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Error("Failed to launch browser from container: ", err)
			return
		}

		s.Logf("Waiting for renderer with URL %q", urlTarget)
		conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(urlTarget))
		if err != nil {
			s.Error("Didn't see crosh renderer: ", err)
		} else {
			conn.Close()
		}
	}

	checkLaunch("http://browser-env.test/", "sh", "-c", "${BROWSER} http://browser-env.test/")
	checkLaunch("http://x-www-browser.test/", "/etc/alternatives/x-www-browser", "http://x-www-browser.test/")
	checkLaunch("http://xdg-open.test/", "xdg-open", "http://xdg-open.test/")
}
