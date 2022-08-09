// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/local/dbusutil"
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
			if err = proc.SendSignal(syscall.SIGTERM); err != nil {
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

	const source = "fusebox://fusebox-alive-test"
	options := []string{"--fake"}
	if err := cd.Mount(ctx, source, "fusebox", options); err != nil {
		s.Fatal("CrosDisks Mount call failed: ", err)
	}
	defer cd.Unmount(cleanupCtx, source, nil /* options */)

	m, err := w.Wait(ctx)
	if err != nil {
		s.Fatal("CrosDisks MountCompleted event failed: ", err)
	}

	// Connect to the fusebox daemon D-Bus interface.
	const (
		dbusName      = "org.chromium.FuseBoxReverseService"
		dbusPath      = "/org/chromium/FuseBoxReverseService"
		dbusInterface = "org.chromium.FuseBoxReverseService"
	)
	_, dbusObj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		s.Fatal("Failed to connect to fusebox service: ", err)
	}

	// Test D-Bus: call fusebox daemon D-Bus TestIsAlive method.
	const method = dbusInterface + ".TestIsAlive"
	var alive bool = false
	err = dbusObj.CallWithContext(ctx, method, 0).Store(&alive)
	if err != nil || !alive {
		s.Fatalf("TestIsAlive failed: %v alive %v", err, alive)
	}

	// Test FUSE request: stat(2) fake file entry "hello".
	hello := filepath.Join(m.MountPath, "hello")
	if _, err := os.Stat(hello); err != nil {
		s.Fatal("Failed stat(2): ", err)
	}
}
