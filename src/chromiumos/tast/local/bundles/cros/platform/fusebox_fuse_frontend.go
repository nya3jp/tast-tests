// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
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
		s.Fatalf("create %s failed: %v", file, err)
	} else {
		f.Close()
	}

	return "create " + file
}

func write(ctx context.Context, s *testing.State, path, file string) string {
	file = filepath.Join(path, file)

	if err := os.WriteFile(file, []byte("bon jour\n"), 0644); err != nil {
		s.Fatalf("write %s failed: %v", file, err)
	}

	return "write " + file
}

func read(ctx context.Context, s *testing.State, path, file string) string {
	file = filepath.Join(path, file)

	data, err := os.ReadFile(file)
	if err != nil {
		s.Fatalf("read %s failed: %v", file, err)
	}

	return "read [" + strings.TrimSpace(string(data)) + "]"
}

func seek(ctx context.Context, s *testing.State, path, file string) string {
	file = filepath.Join(path, file)

	f, err := os.Open(file)
	if err != nil {
		s.Fatalf("open %s failed: %v", file, err)
	}

	if _, err := f.Seek(6, 0); err != nil {
		s.Fatalf("seek %s at 6 bytes failed: %v", file, err)
	}

	data := make([]byte, 5)
	if _, err = f.Read(data); err != nil {
		s.Fatalf("read %s failed: %v", file, err)
	} else {
		f.Close()
	}

	return "seek [" + strings.TrimSpace(string(data)) + "]"
}

func cp(ctx context.Context, s *testing.State, path, src, des string) string {
	src = filepath.Join(path, src)
	des = filepath.Join(path, des)

	const cp = "/bin/cp"
	if _, err := testexec.CommandContext(ctx, cp, src, des).Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("cp %s to %s failed: %v", src, des, err)
	}

	return "cp " + src
}

func mkdir(ctx context.Context, s *testing.State, path, dir string) string {
	dir = filepath.Join(path, dir)

	const mkdir = "/bin/mkdir"
	if _, err := testexec.CommandContext(ctx, mkdir, dir).Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("mkdir %s failed: %v", dir, err)
	}

	return "mkdir " + dir
}

func rmdir(ctx context.Context, s *testing.State, path, dir string) string {
	dir = filepath.Join(path, dir)

	const rm = "/bin/rm" // using the force -rf
	if _, err := testexec.CommandContext(ctx, rm, "-rf", dir).Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("rmdir %s failed: %v", dir, err)
	}

	return "rmdir " + dir
}

func rm(ctx context.Context, s *testing.State, path, file string) string {
	file = filepath.Join(path, file)

	const rm = "/bin/rm"
	if _, err := testexec.CommandContext(ctx, rm, file).Output(testexec.DumpLogOnError); err != nil {
		s.Fatalf("rm %s failed: %v", file, err)
	}

	return "rm " + file
}

func rename(ctx context.Context, s *testing.State, path, src, des string) string {
	src = filepath.Join(path, src)
	des = filepath.Join(path, des)

	const mv = "/bin/mv"
	if output, err := testexec.CommandContext(ctx, mv, src, des).CombinedOutput(); err != nil {
		return fmt.Sprintf("rename ENOTSUP %s", output)
	}

	return "rename " + src
}

func ls(ctx context.Context, s *testing.State, path string) string {
	const ls = "/bin/ls"

	output, err := testexec.CommandContext(ctx, ls, "-ls", "-R", path).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("List %s failed: %v", path, err)
	}

	return strings.TrimSpace("ls " + string(output[:]))
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
	} else if m.MountPath != "/media/fuse/fusebox-fuse-frontend-test" {
		s.Fatal("Failed invalid mount point: ", m.MountPath)
	} else {
		s.Log("CrosDisks mounted ", m.MountPath)
	}

	// Send POSIX API test commands to the fusebox FUSE frontend.
	var output []string
	output = append(output, ls(ctx, s, m.MountPath))
	output = append(output, create(ctx, s, m.MountPath, "file"))
	output = append(output, cp(ctx, s, m.MountPath, "hello", "copy"))
	output = append(output, ls(ctx, s, m.MountPath))
	output = append(output, read(ctx, s, m.MountPath, "copy"))
	output = append(output, write(ctx, s, m.MountPath, "copy"))
	output = append(output, read(ctx, s, m.MountPath, "copy"))
	output = append(output, ls(ctx, s, m.MountPath))
	output = append(output, seek(ctx, s, m.MountPath, "hello"))
	output = append(output, mkdir(ctx, s, m.MountPath, "dir"))
	output = append(output, create(ctx, s, m.MountPath, "dir/file"))
	output = append(output, mkdir(ctx, s, m.MountPath, "dir/dir"))
	output = append(output, ls(ctx, s, m.MountPath))
	output = append(output, rename(ctx, s, m.MountPath, "file", "food"))
	output = append(output, ls(ctx, s, m.MountPath))
	output = append(output, rm(ctx, s, m.MountPath, "file"))
	output = append(output, rmdir(ctx, s, m.MountPath, "dir"))
	output = append(output, ls(ctx, s, m.MountPath))
	output = append(output, rm(ctx, s, m.MountPath, "copy"))
	output = append(output, ls(ctx, s, m.MountPath))

	if false { // Note: set true to see the POSIX command output.
		for i := 0; i < len(output); i++ {
			s.Logf("%s", output[i])
		}
	}
}
