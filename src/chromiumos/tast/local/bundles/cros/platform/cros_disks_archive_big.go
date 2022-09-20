// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	c "chromiumos/tast/local/bundles/cros/platform/crosdisks"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

var PreparedArchives = []string{
	"100,000 Files.7z",
	"100,000 Files.iso",
	"100,000 Files.rar",
	"100,000 Files.tar.xz",
	"100,000 Files.zip",
	"Big One.rar",
	"Big One.tar.xz",
	"Big One.zip",
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     CrosDisksArchiveBig,
		Desc:     "Checks that cros-disks can mount big archives",
		Contacts: []string{"chromeos-files-syd@google.com"},
		Attr:     []string{"group:mainline"},
		Data:     PreparedArchives,
		Timeout:  10 * time.Minute,
	})
}

func testBig(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, dataDir string) {
	for _, archive := range []string{
		"Big One.zip",
		"Big One.rar",
		"Big One.tar.xz",
	} {
		if err := c.WithMountedArchiveDo(ctx, cd, filepath.Join(dataDir, archive), nil, func(ctx context.Context, mountPath string) error {
			// Open file from mounted archive.
			p := filepath.Join(mountPath, "Big One.txt")
			f, err := os.Open(p)
			if err != nil {
				return err
			}

			// Read and hash file from mounted archive using MD5.
			s.Logf("Hashing %q", p)
			h := md5.New()
			n, err := io.Copy(h, f)
			if err != nil {
				return err
			}
			s.Logf("Hashed %q", p)

			// Check file size.
			if want := int64(6777995272); n != want {
				return errors.Errorf("unexpected file size: got %d bytes, want %d bytes", n, want)
			}

			// Check MD5 hash value.
			if got, want := hex.EncodeToString(h.Sum(nil)), "2095613d0172b743430ffca9401c39b6"; got != want {
				return errors.Errorf("unexpected MD5 hash: got %q, want %q", got, want)
			}

			return nil
		}); err != nil {
			s.Errorf("Test failed for %q: %v", archive, err)
		}
	}
}

func testMany(ctx context.Context, s *testing.State, cd *crosdisks.CrosDisks, archiveDir string) error {
	want := c.DirectoryContents{}

	for i := 0; i < 100; i++ {
		for j := 0; j < 1000; j++ {
			want[fmt.Sprintf("Folder %02d/File %02d-%03d.txt", i, i, j)] = c.FileItem{
				// TODO(crbug.com/1335301) Read and check file contents.
				// Data: []byte(fmt.Sprintf("Message %02d-%03d\n", i, j)),
			}
		}
	}

	for _, archive := range []string{
		"100,000 Files.zip",
		"100,000 Files.rar",
		"100,000 Files.7z",
		"100,000 Files.iso",
		"100,000 Files.tar.xz",
	} {
		archivePath := filepath.Join(archiveDir, archive)
		if err := c.VerifyArchiveContent(ctx, cd, archivePath, nil, want); err != nil {
			return errors.Wrapf(err, "test failed for %q", archive)
		}
	}

	return nil
}

func CrosDisksArchiveBig(ctx context.Context, s *testing.State) {
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Cannot connect CrosDisks D-Bus service: ", err)
	}
	defer cd.Close()

	// Create a FAT filesystem containing all our test archive files.
	err = c.WithLoopbackDeviceDo(ctx, cd, 512*1024*1024, "mkfs.vfat -n ARCHIVES", func(ctx context.Context, ld *crosdisks.LoopbackDevice) (err error) {
		// Mounting it through CrosDisks will put the archives where we expect users to have them, so they are already in a permitted location.
		return c.WithMountDo(ctx, cd, ld.DevicePath(), "", []string{"rw"}, func(ctx context.Context, mountPath string, readOnly bool) error {
			if readOnly {
				return errors.Errorf("unexpected read-only flag for %q: got %v; want false", mountPath, readOnly)
			}

			s.Logf("Copying archives to loopback device mounted at %q", mountPath)
			for _, name := range PreparedArchives {
				s.Logf("Copying %q to %q", name, mountPath)
				if err := c.CopyFile(s.DataPath(name), filepath.Join(mountPath, filepath.Base(name))); err != nil {
					return errors.Wrapf(err, "cannot copy file %q into %q", name, mountPath)
				}
			}

			s.Run(ctx, "Big", func(ctx context.Context, state *testing.State) {
				testBig(ctx, state, cd, mountPath)
			})
			s.Run(ctx, "Many", func(ctx context.Context, state *testing.State) {
				testMany(ctx, state, cd, mountPath)
			})
			return nil
		})
	})

	if err != nil {
		s.Fatal("Error while running tests: ", err)
	}
}
