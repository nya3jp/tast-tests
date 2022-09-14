// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Fusebox,
		Desc: "Mount fusebox daemon and verify it responds to requests",
		Contacts: []string{
			"noel@chromium.org",
			"benreich@chromium.org",
			"nigeltao@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func Fusebox(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	terminateDaemonIfNeeded := func(daemon string) {
		if proc, err := procutil.FindUnique(procutil.ByExe(daemon)); err == nil {
			testing.ContextLog(ctx, "Terminating existing daemon")
			if err = proc.SendSignal(unix.SIGTERM); err != nil {
				testing.ContextLog(ctx, "Failed to terminate daemon: ", err)
			}
		}
	}

	terminateDaemonIfNeeded("/usr/bin/fusebox")

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

	const source = "fusebox://fusebox-basic-test"
	if err := cd.Mount(ctx, source, "fusebox", nil); err != nil {
		s.Fatal("CrosDisks Mount call failed: ", err)
	}
	defer cd.Unmount(cleanupCtx, source, nil)

	m, err := w.Wait(ctx)
	if err != nil {
		s.Fatal("CrosDisks MountCompleted event failed: ", err)
	}

	// The "fuse_status" and "ok\n" magic strings are defined in
	// "platform2/fusebox/built_in.cc".
	fuseStatusFilename := filepath.Join(m.MountPath, "built_in/fuse_status")
	if got, err := os.ReadFile(fuseStatusFilename); err != nil {
		s.Fatal("ReadFile failed: ", err)
	} else if want := "ok\n"; string(got) != want {
		s.Fatalf("ReadFile: got %q, want %q", got, want)
	}
}
