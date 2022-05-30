// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Fusebox,
		Desc: "Mount fusebox deamon and verify it can respond to requests",
		Contacts: []string{
			"noel@chromium.org",
			"benreich@chromium.org",
			"chromeos-files-app@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func Fusebox(ctx context.Context, s *testing.State) {
	const (
		dbusName      = "org.chromium.FuseBoxReverseService"
		dbusPath      = "/org/chromium/FuseBoxReverseService"
		dbusInterface = "org.chromium.FuseBoxReverseService"
	)

	s.Log("Connecting to CrosDisks D-Bus service")
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed connecting to CrosDisks D-Bus service: ", err)
	}
	defer cd.Close()

	w, err := cd.WatchMountCompleted(ctx)
	if err != nil {
		s.Fatal("Failed to get MountCompleted event watcher: ", err)
	}
	defer w.Close(ctx)

	const source = "fusebox://fusebox-tast-test-requests"
	options := []string{"--fake", "--debug", "--v=2"}

	s.Logf("Mounting fusebox source: %q", source)
	if err := cd.Mount(ctx, source, "fusebox", options); err != nil {
		s.Error("Failed during CrosDisks Mount call: ", err)
		return
	}
	defer cd.Unmount(ctx, source, nil /* options */)

	s.Log("Awaiting MountCompleted event")
	m, err := w.Wait(ctx)
	if err != nil {
		s.Error("Failed awaiting MountCompleted event: ", err)
		return
	}

	s.Logf("CrosDisks mounted %q at %v", m.SourcePath, m.MountPath)
	if m.SourcePath != source {
		s.Error("Failed: invalid source: ", m.SourcePath)
		return
	} else if m.MountPath != "/media/fuse/fusebox-tast-test-requests" {
		s.Error("Failed: invalid mount-point: ", m.MountPath)
		return
	}

	// Test FUSE request: stat(2) fake file system file entry "hello".
	hello := filepath.Join(m.MountPath, "hello")
	s.Log("Calling stat(2) on fusebox fake file entry ", hello)
	if _, err := os.Stat(hello); err != nil {
		s.Error("Failed stat(2): ", err)
		return
	}

	// Test D-BUS request: call fusebox daemon D-BUS interface method.
	s.Logf("Connecting to D-Bus interface %s", dbusName)
	_, dbusObj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		s.Errorf("Failed to connect to interface %s: %v", dbusName, err)
		return
	}

	var result bool
	s.Log("Calling fusebox D-Bus interface TestIsAlive method")
	if err := dbusObj.CallWithContext(ctx, dbusInterface+".TestIsAlive", 0).Store(&result); err != nil {
		s.Error("Failed to call D-Bus TestIsAlive method: ", err)
		return
	} else if !result {
		s.Error("Failed: D-Bus TestIsAlive method returned false")
		return
	}
}
