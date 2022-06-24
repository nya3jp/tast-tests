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
		Func: FuseboxAttachStorage, // go/fusebox-tast-tests
		Desc: "Mount fusebox daemon and test {Attach,Detach}Storage APIs",
		Contacts: []string{
			"noel@chromium.org",
			"benreich@chromium.org",
			"chromeos-files-syd@google.com",
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
		s.Fatal("Failed to connect to CrosDisks service: ", err)
	}
	defer cd.Close()

	w, err := cd.WatchMountCompleted(ctx)
	if err != nil {
		s.Fatal("Failed to create MountCompleted event watcher: ", err)
	}
	defer w.Close(cleanupCtx)

	const source = "fusebox://fusebox-storage-test"
	if err := cd.Mount(ctx, source, "fusebox", nil /* options */); err != nil {
		s.Fatal("CrosDisks Mount failed: ", err)
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

	// Attach storage device to the mount point.
	const attach = dbusInterface + ".AttachStorage"
	const device = "mtp filesystem:url ro"
	var result int32 = -1
	err = dbusObj.CallWithContext(ctx, attach, 0, device).Store(&result)
	if err != nil || result != 0 {
		s.Fatalf("AttachStorage failed: %v error %d", err, result)
	}

	// Verify the device attached to the mount point.
	files, err := os.ReadDir(m.MountPath)
	if err != nil {
		s.Fatal("Failed reading mount point: ", err)
	}
	if _, err := os.Stat(filepath.Join(m.MountPath, "mtp")); err != nil {
		s.Fatal("Failed stat(2): ", err)
	}

	// Detach storage device from the mount point.
	const detach = dbusInterface + ".DetachStorage"
	result = -1
	err = dbusObj.CallWithContext(ctx, detach, 0, "mtp").Store(&result)
	if err != nil || result != 0 {
		s.Fatalf("DetachStorage failed: %v error %d", err, result)
	}

	// Verify the device detached from the mount point.
	length := len(files) - 1
	files, err = os.ReadDir(m.MountPath)
	if err != nil || len(files) != length {
		s.Fatalf("DetachStorage failed: %v files %d", err, len(files))
	}
}
