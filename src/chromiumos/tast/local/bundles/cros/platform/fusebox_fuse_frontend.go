// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FuseboxFuseFrontend,
		Desc: "Mount fusebox daemon and test FUSE frontend POSIX operations",
		Contacts: []string{
			"noel@chromium.org",
			"benreich@chromium.org",
			"chromeos-files-app@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func create(ctx context.Context, s *testing.State, path, file string) string {
	file = filepath.Join(path, file)

	if f, err := os.Create(file); err != nil {
		s.Fatalf("Copy %s failed: %v", file, err)
	} else {
		f.Close()
	}

	return "Create " + file
}

func write(ctx context.Context, s *testing.State, path, file string) string {
	file = filepath.Join(path, file)

	if err := os.WriteFile(file, []byte("allo there\n"), 0644); err != nil {
		s.Fatalf("Write %s failed: %v", file, err)
	}

	return "Write " + file
}

func read(ctx context.Context, s *testing.State, path, file string) string {
	file = filepath.Join(path, file)

	data, err := os.ReadFile(file)
	if err != nil {
		s.Fatalf("Read %s failed: %v", file, err)
	}

	return strings.TrimSpace("Read " + string(data))
}

func seek(ctx context.Context, s *testing.State, path, file string) string {
	file = filepath.Join(path, file)

	f, err := os.Open(file)
	if err != nil {
		s.Fatalf("Open %s failed: %v", file, err)
	}

	if off, err := f.Seek(6, 0); err != nil {
		s.Fatalf("Seek %s at %d failed: %v", file, off, err)
	}

	data := make([]byte, 5)
	if _, err = f.Read(data); err != nil {
		s.Fatalf("Read %s failed: %v", file, err)
	} else {
		f.Close()
	}

	return strings.TrimSpace("Seek " + string(data))
}

func copy(ctx context.Context, s *testing.State, path, src, des string) string {
	src = filepath.Join(path, src)
	des = filepath.Join(path, des)

	const copy = "/bin/cp"
	if _, err := testexec.CommandContext(ctx, copy, src, des).Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Copy %s to %s failed: %v", src, des, err)
	}

	return "Copy " + src
}

func mkDir(ctx context.Context, s *testing.State, path, dir string) string {
	dir = filepath.Join(path, dir)

	const mkdir = "/bin/mkdir"
	if _, err := testexec.CommandContext(ctx, mkdir, dir).Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("MkDir %s failed: %v", dir, err)
	}

	return "MkDir " + dir
}

func rmDir(ctx context.Context, s *testing.State, path, dir string) string {
	dir = filepath.Join(path, dir)

	const rm = "/bin/rm"
	if _, err := testexec.CommandContext(ctx, rm, "-rf", dir).Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("RmDir %s failed: %v", dir, err)
	}

	return "RmDir " + dir
}

func rmFile(ctx context.Context, s *testing.State, path, file string) string {
	file = filepath.Join(path, file)

	const rm = "/bin/rm"
	if _, err := testexec.CommandContext(ctx, rm, file).Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("RmFile %s failed: %v", file, err)
	}

	return "RmFile " + file
}

func list(ctx context.Context, s *testing.State, path string) string {
	const list = "/bin/ls"

	output, err := testexec.CommandContext(ctx, list, "-Rls", path).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("List %s failed: %v", path, err)
	}

	return strings.TrimSpace("List " + string(output[:]))
}

func FuseboxFuseFrontend(ctx context.Context, s *testing.State) {
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

	const source = "fusebox://fusebox-fuse-frontend-test"
	options := []string{"--fake", "--debug", "--v=2"}
	if err := cd.Mount(ctx, source, "fusebox", options); err != nil {
		s.Fatal("CrosDisks Mount call failed: ", err)
	}
	defer cd.Unmount(cleanupCtx, source, nil /* options */)

	m, err := w.Wait(ctx)
	if err != nil {
		s.Fatal("Failed awaiting MountCompleted event: ", err)
	} else if m.SourcePath != source {
		s.Fatal("Failed invalid mount source: ", m.SourcePath)
	} else if m.MountPath != "/media/fuse/fusebox-fuse-frontend-test" {
		s.Fatal("Failed invalid mount point: ", m.MountPath)
	} else {
		s.Log("CrosDisks mounted ", m.MountPath)
	}

	var output []string
	output = append(output, list(ctx, s, m.MountPath))
	output = append(output, create(ctx, s, m.MountPath, "create"))
	output = append(output, copy(ctx, s, m.MountPath, "hello", "copy"))
	output = append(output, list(ctx, s, m.MountPath))
	output = append(output, read(ctx, s, m.MountPath, "copy"))
	output = append(output, write(ctx, s, m.MountPath, "copy"))
	output = append(output, read(ctx, s, m.MountPath, "copy"))
	output = append(output, list(ctx, s, m.MountPath))
	output = append(output, seek(ctx, s, m.MountPath, "hello"))
	output = append(output, mkDir(ctx, s, m.MountPath, "dir"))
	output = append(output, Create(ctx, s, m.MountPath, "dir/child"))
	output = append(output, mkDir(ctx, s, m.MountPath, "dir/foo"))
	output = append(output, list(ctx, s, m.MountPath))
	output = append(output, rmDir(ctx, s, m.MountPath, "dir"))
	output = append(output, list(ctx, s, m.MountPath))
	output = append(output, rmFile(ctx, s, m.MountPath, "copy"))
	output = append(output, rmFile(ctx, s, m.MountPath, "create"))
	output = append(output, list(ctx, s, m.MountPath))

	if false { // set true to see the command outputs
		for i := 0; i < len(output); i++ {
			s.Logf("%s", output[i])
		}
	}
}
