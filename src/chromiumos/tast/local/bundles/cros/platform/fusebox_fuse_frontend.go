// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FuseboxFuseFrontend, // go/fusebox-tast-tests
		Desc: "Mount fusebox and test its FUSE frontend using POSIX file API",
		Vars: []string{"platform.FuseboxFuseFrontend.debug"},
		Contacts: []string{
			"noel@chromium.org",
			"benreich@chromium.org",
			"chromeos-files-app@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func FuseboxFuseFrontend(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to CrosDisks service: ", err)
	}
	defer cd.Close()

	w, err := cd.WatchMountCompleted(ctx)
	if err != nil {
		s.Fatal("Failed to create MountCompleted event watcher: ", err)
	}
	defer w.Close(cleanupCtx)

	const source = "fusebox://fusebox-fuse-frontend-test"
	options := []string{"--fake"}
	if err := cd.Mount(ctx, source, "fusebox", options); err != nil {
		s.Fatal("CrosDisks Mount call failed: ", err)
	}
	defer cd.Unmount(cleanupCtx, source, nil /* options */)

	m, err := w.Wait(ctx)
	if err != nil {
		s.Fatal("CrosDisks MountCompleted event failed: ", err)
	}

	// Run command: enable tast debug -var to see command output.
	_, debug := s.Var("platform.FuseboxFuseFrontend.debug")
	run := func(command string) {
		command = strings.ReplaceAll(command, "MP", m.MountPath)
		args := strings.Fields(command)
		exec := testexec.CommandContext(ctx, args[0], args[1:]...)
		if output, err := exec.Output(testexec.DumpLogOnError); err != nil {
			s.Fatalf("%s failed: %v", command, err)
		} else if debug {
			s.Logf("%s %s", command, output)
		}
	}

	// Send POSIX API test commands to the fusebox FUSE frontend.
	run("/bin/ls -lsR    MP")
	run("/bin/touch      MP/file")
	run("/bin/cp         MP/hello MP/copy")
	run("/bin/cat        MP/copy")
	run("/bin/dd         if=/dev/zero of=MP/copy ibs=1 count=919")
	run("/bin/ls -lsR    MP")
	run("/bin/mkdir -p   MP/dir/dir")
	run("/bin/cp         MP/hello MP/dir/file")
	run("/bin/ls -lsR    MP")
	run("/bin/mv         MP/file MP/move")
	run("/bin/mv         MP/dir/dir MP/here")
	run("/bin/ls -lsR    MP")
	run("/bin/rm -rf     MP/here MP/dir MP/move MP/copy")
}
