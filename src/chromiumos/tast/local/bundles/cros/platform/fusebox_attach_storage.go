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
		Func: FuseboxAttachStorage,
		Desc: "Mount fusebox daemon and test {Attach,Detach}Storage APIs",
		Contacts: []string{
			"noel@chromium.org",
			"benreich@chromium.org",
			"chromeos-files-app@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func FuseboxAttachStorage(ctx context.Context, s *testing.State) {
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

	const source = "fusebox://fusebox-storage-test"
	if err := cd.Mount(ctx, source, "fusebox", nil); err != nil {
		s.Fatal("CrosDisks Mount call failed: ", err)
	}
	defer cd.Unmount(cleanupCtx, source, nil /* options */)

	m, err := w.Wait(ctx)
	if err != nil {
		s.Fatal("Failed awaiting MountCompleted event: ", err)
	} else if m.SourcePath != source {
		s.Fatal("Failed invalid mount source: ", m.SourcePath)
	} else if m.MountPath != "/media/fuse/fusebox-storage-test" {
		s.Fatal("Failed invalid mount point: ", m.MountPath)
	} else {
		s.Log("CrosDisks mounted ", m.MountPath)
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

	// Attach storage device to the mount point.
	const attach = dbusInterface + "." + "AttachStorage"
	const name = "mtp filesystem:url ro"
	var result int32 = -1
	s.Logf("Calling %s %q", attach, name)
	if err := dbusObj.CallWithContext(ctx, attach, 0, name).Store(&result); err != nil {
		s.Fatalf("Failed calling %s: %v", attach, err)
	} else if result != 0 {
		s.Fatalf("Failed %s returned: %d", attach, result)
	}

	// Verify the device attached to the mount point.
	files, err := os.ReadDir(m.MountPath)
	if err != nil {
		s.Fatal("Failed reading moint point: ", err)
	} else if _, err := os.Stat(filepath.Join(m.MountPath, "mtp")); err != nil {
		s.Fatal("Failed stat(2): ", err)
	}

	// Detach storage device from the mount point.
	count := len(files)
	const detach = dbusInterface + "." + "DetachStorage"
	s.Logf("Calling %s %q", detach, "mtp")
	result = -1
	if err := dbusObj.CallWithContext(ctx, detach, 0, "mtp").Store(&result); err != nil {
		s.Fatalf("Failed calling %s: %v", detach, err)
	} else if result != 0 {
		s.Fatalf("Failed %s returned: %d", detach, result)
	}

	// Verify the device detached from the mount point.
	files, err = os.ReadDir(m.MountPath)
	if err != nil || len(files) != (count-1) {
		s.Fatalf("Failed reading moint point: %v files %d", err, len(files))
	}
}
