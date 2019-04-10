// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MountOBB,
		Desc: "Verifies mount-obb's fuse works",
		Contacts: []string{
			"hashimoto@chromium.org", // original author.
			"arc-storage@google.com",
			"hidehiko@chromium.org", // Tast port.
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android"},
		Timeout:      4 * time.Minute,
	})
}

func MountOBB(ctx context.Context, s *testing.State) {
	largeData := bytes.Repeat([]byte("abcdefghijklmnopqrstuvwxyz"), 12345)

	// createFile creates a file at path with given content.
	createFile := func(path string, content []byte) error {
		if err := ioutil.WriteFile(path, content, 0644); err != nil {
			return errors.Wrapf(err, "failed to create %s", path)
		}
		return nil
	}

	// setUpFiles creates files to be tested.
	// - removed${I}.txt: once created, then removed later.
	// - file${I}.txt: created with containing "${I}" as its contents.
	// - large_file.data: created with largeData defined above.
	// where ${I} will be 0..99 inclusively.
	setUpFiles := func(dir string) error {
		// Create "removed" files and "file" files.
		for i := 0; i < 100; i++ {
			content := []byte(strconv.Itoa(i))
			if err := createFile(filepath.Join(dir, fmt.Sprintf("removed%d.txt", i)), content); err != nil {
				return err
			}

			if err := createFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), content); err != nil {
				return err
			}
		}

		// Create a file with large content.
		if err := createFile(filepath.Join(dir, "large_file.data"), largeData); err != nil {
			return err
		}

		// Remove "removed" files.
		for i := 0; i < 100; i++ {
			path := filepath.Join(dir, fmt.Sprintf("removed%d.txt", i))
			if err := os.Remove(path); err != nil {
				return errors.Wrapf(err, "failed to remove %s", path)
			}
		}
		return nil
	}

	verifyFiles := func(dir string) {
		// Verify existing files have expected content.
		for i := 0; i < 100; i++ {
			path := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
			expect := []byte(strconv.Itoa(i))
			if data, err := ioutil.ReadFile(path); err != nil {
				s.Errorf("Failed to read %s: %v", path, err)
			} else if !bytes.Equal(data, expect) {
				s.Errorf("Unexpected contents in %s: got %q; want %q", path, data, expect)
			}

		}

		// Verify removed files do not exist.
		for i := 0; i < 100; i++ {
			path := filepath.Join(dir, fmt.Sprintf("removed%d.txt", i))
			if _, err := os.Stat(path); err == nil {
				s.Error("Unexpected file exists: ", path)
			} else if !os.IsNotExist(err) {
				s.Errorf("Stat(%q) failed: %v", path, err)
			}
		}

		// Verify the large file has expected content.
		path := filepath.Join(dir, "large_file.data")
		if data, err := ioutil.ReadFile(path); err != nil {
			s.Errorf("Failed to read %s: %v", path, err)
		} else if !bytes.Equal(data, largeData) {
			// Because data and largeData is huge, do not print in error.
			s.Errorf("Large data mismatch for %s", path)
		}
	}

	setUpImage := func(fatType int, tempdir, path string) error {
		if err := testexec.CommandContext(ctx, "truncate", "--size=64M", path).Run(testexec.DumpLogOnError); err != nil {
			return err
		}

		if err := testexec.CommandContext(ctx, "mkfs.vfat", "-F", strconv.Itoa(fatType), path).Run(testexec.DumpLogOnError); err != nil {
			return err
		}

		mountPath := filepath.Join(tempdir, "setup_mount")
		if err := os.MkdirAll(mountPath, 0755); err != nil {
			return errors.Wrap(err, "failed to create a dir at "+mountPath)
		}
		if err := testexec.CommandContext(ctx, "mount", path, mountPath).Run(testexec.DumpLogOnError); err != nil {
			return err
		}
		defer syscall.Unmount(mountPath, syscall.MNT_DETACH)

		// Put files in the root directory.
		if err := setUpFiles(mountPath); err != nil {
			return err
		}

		// Put files in a child directory.
		subdirPath := filepath.Join(mountPath, "foo", "bar")
		if err := os.MkdirAll(subdirPath, 0755); err != nil {
			return errors.Wrap(err, "failed to create dir at "+subdirPath)
		}
		if err := setUpFiles(subdirPath); err != nil {
			return err
		}

		return nil
	}

	runTest := func(fatType int) {
		s.Logf("Testing FAT%d", fatType)

		tempdir, err := ioutil.TempDir("", "fat"+strconv.Itoa(fatType))
		if err != nil {
			s.Error("Failed to create tempdir: ", err)
			return
		}
		defer os.RemoveAll(tempdir)
		defer func() {
			ms, err := sysutil.MountInfoForPID(0)
			if err != nil {
				s.Log("Failed to list mount points: ", err)
				return
			}
			for _, m := range ms {
				if !strings.HasPrefix(m.MountPath, tempdir) {
					continue
				}
				if err := syscall.Unmount(m.MountPath, syscall.MNT_DETACH); err != nil {
					s.Logf("Failed to unmount %s: %v", m.MountPath, err)
				}
			}
		}()

		fatFile := filepath.Join(tempdir, "img")
		if err := setUpImage(fatType, tempdir, fatFile); err != nil {
			s.Errorf("Failed to create FAT%d image: %v", fatType, err)
			return
		}

		mountPath := filepath.Join(tempdir, "mount")
		if err := os.MkdirAll(mountPath, 0755); err != nil {
			s.Error("Failed to creat a tempdir: ", err)
			return
		}

		cmd := testexec.CommandContext(ctx, "mount-obb", fatFile, mountPath, "0" /* root UID */, "0" /* root GID */)
		if err := cmd.Start(); err != nil {
			s.Error("Failed to start mount-obb: ", err)
			return
		}
		defer func() {
			cmd.Kill()
			cmd.Wait()
		}()
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			ms, err := sysutil.MountInfoForPID(0)
			if err != nil {
				return testing.PollBreak(err)
			}
			for _, m := range ms {
				if strings.HasPrefix(m.MountPath, tempdir) {
					return nil
				}
			}
			return errors.New("mount point is not yet ready")
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Error("Mount point did not become available: ", err)
			return
		}

		verifyFiles(mountPath)
		verifyFiles(filepath.Join(mountPath, "foo", "bar"))
	}

	runTest(12) // Test FAT12
	runTest(16) // Test FAT16
	runTest(32) // Test FAT32
}
