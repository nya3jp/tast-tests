// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchTerminal,
		Desc:         "Executes the x-terminal-emulator alternative in the container which should then cause Chrome to open the Terminal extension",
		Contacts:     []string{"davidmunro@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Vars:         []string{"keepState"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params:       crostini.MakeTestParams(crostini.TestInformational),
	})
}

func LaunchTerminal(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	cont := pre.Container
	defer crostini.RunCrostiniPostTest(ctx, cont)

	const terminalURLContains = ".html?command=vmshell"

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
		conn, err := cr.NewConnForTarget(ctx, func(t *target.Info) bool {
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
