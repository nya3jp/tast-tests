// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniFiles,
		Desc:         "Checks that crostini sshfs integration with FilesApp works",
		Attr:         []string{"informational"},
		Timeout:      300 * time.Second,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func CrostiniFiles(s *testing.State) {
	cr, err := chrome.New(s.Context())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	tconn, err := cr.TestAPIConn(s.Context())
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	if err = tconn.Exec(s.Context(), "chrome.autotestPrivate.installCrostini()"); err != nil {
		s.Fatal("Running autotestPrivate.installCrostini failed: ", err)
	}

	// Wait until sshfs mount is detected at /media/fuse/crostini_<hash>_termina_penguin.
	cmd := testexec.CommandContext(s.Context(), "sh", "-c", "while [ ! -d /media/fuse/crostini_*_termina_penguin ]; do sleep 0.1; done")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(s.Context())
		s.Fatal("Failed waiting for sshfs mount: ", err)
	}

	// Verify mount works for copying a file.
	cmd = testexec.CommandContext(s.Context(), "sh", "-c", "echo hello > `ls -d  /media/fuse/crostini_*_termina_penguin`/hello.txt")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(s.Context())
		s.Fatal("Failed waiting for sshfs mount: ", err)
	}

	// TODO(joehockey): Use terminal app to verify hello.txt.
}
