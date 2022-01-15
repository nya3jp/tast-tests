// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ModuleLocking,
		Desc: "Checks that kernel modules can't be loaded from outside the root filesystem",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func ModuleLocking(ctx context.Context, s *testing.State) {
	const (
		sysctl        = "/proc/sys/kernel/chromiumos/module_locking"
		module        = "test_module"                  // installed in test images
		moduleFile    = "kernel/lib/test_module.ko"    // uncompressed version
		altModuleFile = "kernel/lib/test_module.ko.gz" // compressed version
	)

	s.Log("Checking ", sysctl)
	if b, err := ioutil.ReadFile(sysctl); err != nil && !os.IsNotExist(err) {
		s.Fatalf("Failed to read %s: %v", sysctl, err)
	} else if err == nil && string(b) != "1\n" {
		s.Fatalf("%v contains %q; want 1", sysctl, string(b))
	}

	u, err := sysutil.Uname()
	if err != nil {
		s.Fatal("Failed to get kernel release: ", err)
	}
	moduleDir := filepath.Join("/lib/modules", u.Release)
	var modulePath string
	for _, fn := range []string{moduleFile, altModuleFile} {
		p := filepath.Join(moduleDir, fn)
		if _, err := os.Stat(p); err == nil {
			modulePath = p
			break
		}
	}
	if modulePath == "" {
		s.Fatalf("Failed to find %q module in %s: %v", module, moduleDir, err)
	}
	s.Log("Using ", modulePath)

	// Runs the supplied command. An test error is reported if the result doesn't match wantSuccess.
	run := func(wantSuccess bool, name string, args ...string) {
		cmd := testexec.CommandContext(ctx, name, args...)
		if err := cmd.Run(); err != nil && wantSuccess {
			s.Errorf("%q failed: %v", strings.Join(cmd.Args, " "), err)
			cmd.DumpLog(ctx)
		} else if err == nil && !wantSuccess {
			s.Errorf("%q unexpectedly succeeded", strings.Join(cmd.Args, " "))
		}
	}

	unloadModule(ctx, s, module)

	s.Log("Attempting to modprobe ", module)
	run(true, "modprobe", module)
	unloadModule(ctx, s, module)

	s.Log("Attempting to insmod ", modulePath)
	run(true, "insmod", modulePath)
	unloadModule(ctx, s, module)

	td, err := ioutil.TempDir("", "security.ModuleLocking.")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(td)

	tmpPath := filepath.Join(td, filepath.Base(modulePath))
	copyModule(s, modulePath, tmpPath, false /* gzip */)
	s.Log("Attempting to insmod ", tmpPath)
	run(false, "insmod", tmpPath)
	unloadModule(ctx, s, module)

	if loadViaInitModule(s, modulePath) {
		s.Error("old-style init_module syscall unexpectedly succeeded")
	}
	unloadModule(ctx, s, module)

	// Guard against a regression of http://b/21762937, where a bind unmount would
	// incorrectly trigger protections against unmounts of pinned filesystems.
	s.Log("Bind-mounting/unmounting and attempting to modprobe ", module)
	mountPoint := filepath.Join(td, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		s.Fatalf("Failed to create %v: %v", mountPoint, err)
	}
	run(true, "mount", "-o", "bind", "/", mountPoint)
	run(true, "umount", mountPoint)
	run(true, "modprobe", module)
	unloadModule(ctx, s, module)
}

// moduleLoaded returns true if the named kernel module (e.g. "test_module") is currently loaded.
// Errors cause a fatal test error to be reported via s.
func moduleLoaded(s *testing.State, module string) bool {
	const modulesPath = "/proc/modules"
	b, err := ioutil.ReadFile(modulesPath)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", modulesPath, err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if parts := strings.Fields(line); len(parts) > 0 && parts[0] == module {
			return true
		}
	}
	return false
}

// unloadModule unloads the named kernel module if it's currently loaded.
// This function is a no-op if the module isn't already loaded.
// Errors cause a fatal test error to be reported via s.
func unloadModule(ctx context.Context, s *testing.State, module string) {
	if !moduleLoaded(s, module) {
		return
	}

	cmd := testexec.CommandContext(ctx, "rmmod", module)
	if err := cmd.Run(); err != nil {
		defer cmd.DumpLog(ctx)
		s.Fatalf("Failed to run %q: %v", strings.Join(cmd.Args, " "), err)
	}
}

// copyModule copies the kernel module file at srcPath to dstPath.
// If useGzip is true, the file is gzip-compressed as it is copied.
// Errors cause a fatal test error to be reported via s.
func copyModule(s *testing.State, srcPath, dstPath string, useGzip bool) {
	src, err := os.Open(srcPath)
	if err != nil {
		s.Fatalf("Failed to open %v: %v", srcPath, err)
	}
	defer src.Close()

	var dst io.WriteCloser
	if dst, err = os.Create(dstPath); err != nil {
		s.Fatalf("Failed to create %v: %v", dstPath, err)
	}

	if useGzip {
		// Wrap the dest file and check that the gzip.Writer closes successfully instead.
		defer dst.Close()
		dst = gzip.NewWriter(dst)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		s.Fatalf("Failed to copy from %v to %v: %v", srcPath, dstPath, err)
	}
	if err := dst.Close(); err != nil {
		s.Fatalf("Failed to close %v: %v", dstPath, err)
	}
}

func loadViaInitModule(s *testing.State, modulePath string) bool {
	var r io.ReadCloser
	r, err := os.Open(modulePath)
	if err != nil {
		s.Fatalf("Failed to open module %q: %v", modulePath, err)
	}
	defer r.Close()

	if filepath.Ext(modulePath) == ".gz" {
		if r, err = gzip.NewReader(r); err != nil {
			s.Fatal("Failed to create gzip reader: ", err)
		}
		defer r.Close()
	}

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		s.Fatal("Failed to read module data: ", err)
	}

	return unix.InitModule(buf, "") == nil
}
