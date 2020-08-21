// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenWithTerminal,
		Desc:         "Open directory in FilesApp with terminal",
		Contacts:     []string{"joelhockey@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Vars:         []string{"keepState"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniStable,
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "artifact_unstable",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:      "download_stretch",
				Pre:       crostini.StartedByDownloadStretch(),
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

func OpenWithTerminal(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	defer crostini.RunCrostiniPostTest(ctx, pre.Container)

	// Launch Files app and open Downloads with terminal.
	filesApp, err := filesapp.Launch(ctx, pre.TestAPIConn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(ctx)

	if err := filesApp.SelectContextMenu(ctx, "Downloads", "Open with Terminal"); err != nil {
		s.Fatal("Open with Terminal failed: ", err)
	}

	// Find terminal window.
	const terminalURLContains = ".html?command=vmshell"
	conn, err := pre.Chrome.NewConnForTarget(ctx, func(t *target.Info) bool {
		return strings.Contains(t.URL, terminalURLContains)
	})
	if err != nil {
		s.Fatal("Couldn't find terminal window: ", err)
	}

	// Validate window title and first row of terminal text.
	const (
		title  = "testuser@penguin: /mnt/chromeos/MyFiles/Downloads"
		prompt = "testuser@penguin:/mnt/chromeos/MyFiles/Downloads$ "
	)
	waitFor := func(expr, want string) {
		if err := conn.WaitForExprFailOnErrWithTimeout(ctx, fmt.Sprintf("%s === '%s'", expr, want), 10*time.Second); err != nil {
			var got string
			if err := conn.Eval(ctx, expr, &got); err != nil {
				s.Logf("Couldn't read %s: %v", expr, err)
			}
			s.Fatalf("Unexpected %q want: %s, got: %s : %v", expr, want, got, err)
		}
	}
	waitFor("document.title", title)
	waitFor("window.term_ && term_.getRowNode(0).textContent", prompt)
	conn.CloseTarget(ctx)
	conn.Close()
}
