// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/local/dbusutil"
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
		Attr: []string{"group:mainline", "informational"},
	})
}

func Fusebox(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed connecting to CrosDisks D-Bus service: ", err)
	}
	defer cd.Close()

	w, err := cd.WatchMountCompleted(ctx)
	if err != nil {
		s.Fatal("Failed to get MountCompleted event watcher: ", err)
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
		s.Fatal("Failed awaiting MountCompleted event: ", err)
	} else if m.SourcePath != source {
		s.Fatal("Failed invalid mount source: ", m.SourcePath)
	} else if m.MountPath != "/media/fuse/fusebox-alive-test" {
		s.Fatal("Failed invalid mount point: ", m.MountPath)
	} else {
		s.Log("CrosDisks mounted ", m.MountPath)
	}

	// Test FUSE request: stat(2) fake file entry "hello".
	hello := filepath.Join(m.MountPath, "hello")
	s.Log("Stating fusebox file entry ", hello)
	if _, err := os.Stat(hello); err != nil {
		s.Fatal("Failed stat(2): ", err)
	}

	// Connect to the fusebox daemon D-Bus interface.
	const (
		dbusName      = "org.chromium.FuseBoxReverseService"
		dbusPath      = "/org/chromium/FuseBoxReverseService"
		dbusInterface = "org.chromium.FuseBoxReverseService"
	)
	_, dbusObj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		s.Fatal("Failed to connect to fusebox D-Bus: ", err)
	}

	// Test D-Bus request: call fusebox daemon D-Bus method.
	const method = dbusInterface + "." + "TestIsAlive"
	s.Logf("Calling %s method", method)
	var result bool = false
	if err := dbusObj.CallWithContext(ctx, method, 0).Store(&result); err != nil {
		s.Fatal("Failed to call D-Bus method: ", err)
	} else if !result {
		s.Fatal("Failed D-Bus method returned false")
	}
}
