// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"runtime/debug"

	"golang.org/x/sys/unix"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Mprotect,
		Desc: "Verifies that mprotect with PROT_EXEC works on noexec mounts",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func Mprotect(ctx context.Context, s *testing.State) {
	// Panic instead of crashing if a fault occurs (which can happen easily when using mmap).
	// This setting applies only to the current goroutine.
	debug.SetPanicOnFault(true)

	// We need a noexec mount for this test to make sense.
	const dir = "/run"
	st := unix.Statfs_t{}
	if err := unix.Statfs(dir, &st); err != nil {
		s.Fatalf("Failed to stat %v: %v", dir, err)
	}
	if st.Flags&unix.MS_NOEXEC == 0 {
		s.Fatal(dir, " not mounted noexec")
	}

	// Create a temp file and write a byte at an offset to zero-fill the earlier portion.
	f, err := ioutil.TempFile(dir, "tast.security.Mprotect.")
	if err != nil {
		s.Fatal("Failed to create temp file: ", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if _, err := f.Seek(100, 1); err != nil {
		s.Fatal("Failed to seek: ", err)
	}
	if _, err := f.Write([]byte{'A'}); err != nil {
		s.Fatal("Failed to write: ", err)
	}

	fd := int(f.Fd())
	data := []byte{0xfa, 0xbe, 0xca, 0xfe} // arbitrary

	// An RW mmap should succeed.
	rw, err := unix.Mmap(fd, 0, len(data),
		unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		s.Fatal("mmap with PROT_READ|PROT_WRITE failed: ", err)
	}
	defer unix.Munmap(rw)
	if !bytes.Equal(rw, make([]byte, len(data))) {
		s.Errorf("Data %v not initially zero-filled", rw)
	}

	// PROT_EXEC should be disallowed since /run is noexec.
	if exe, err := unix.Mmap(fd, 0, len(data),
		unix.PROT_READ|unix.PROT_EXEC, unix.MAP_SHARED); err == nil {
		s.Error("mmap with PROT_READ|PROT_EXEC incorrectly allowed")
		unix.Munmap(exe)
	}

	// An RO mmap should succeed.
	ro, err := unix.Mmap(fd, 0, len(data), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		s.Fatal("mmap with PROT_READ failed: ", err)
	}
	defer unix.Munmap(ro)

	// Using mprotect to set PROT_EXEC should be allowed after the file is mapped.
	// This is needed by NaCl and is enabled by setting /proc/sys/vm/mmap_noexec_taint to 0.
	if err := unix.Mprotect(ro, unix.PROT_READ|unix.PROT_EXEC); err != nil {
		s.Error("mprotect with PROT_READ|PROT_EXEC failed: ", err)
	}

	// After writing to the RW mapping, the RO mapping should show the same contents.
	copy(rw, data)
	if !bytes.Equal(ro, data) {
		s.Errorf("RO map has %v after writing %v to RW map", ro, rw)
	}
}
