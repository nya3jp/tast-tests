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

// LaunchTerminal executes the x-terminal-emulator alternative in the container
// which should then cause Chrome to open the Terminal extension.
func LaunchTerminal(s *testing.State, cr *chrome.Chrome, cont *vm.Container) {
	s.Log("Executing LaunchTerminal test")

	const terminalUrlPrefix = "chrome-extension://nkoccljplnhpfnfiajclkommnmllphnl/html/crosh.html?command=vmshell"

	checkLaunch := func(urlSuffix string, command ...string) {
		ctx, cancel := context.WithTimeout(s.Context(), 10*time.Second)
		defer cancel()

		cmd := cont.Command(s.Context(), command...)
		s.Log(append([]string{"Running: "}, cmd.Args...))
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(s.Context())
			s.Error("Failed to launch terminal command in container: ", err)
			return
		}

		s.Logf("Waiting for renderer with URL prefix %q and suffix %q", terminalUrlPrefix, urlSuffix)
		conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
			return strings.HasPrefix(t.URL, terminalUrlPrefix) &&
				strings.HasSuffix(t.URL, urlSuffix)
		})
		if err != nil {
			s.Error(err)
		} else {
			conn.Close()
		}
	}

	checkLaunch("", "x-terminal-emulator")

	// When we pass an argument to the x-terminal-emulator alternative, it should
	// then append that as URL parameters which will cause the terminal to
	// execute that command initially.
	checkLaunch("&args[]=--&args[]=vim", "x-terminal-emulator", "vim")
}
