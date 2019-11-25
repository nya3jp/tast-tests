// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"encoding/binary"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pkg/xattr"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const userHome = "/home/user"
const rootHome = "/home/root"

const mib uint64 = 1024 * 1024                     // 1 MiB
const gib uint64 = 1024 * mib                      // 1 GiB
const minimalFreeSpace = 512 * mib                 // hard-coded in cryptohomed
const cleanupTrigger = 2 * gib                     // hard-coded in cryptohomed
const startingFreeSpace = cleanupTrigger + 200*mib // 2.2 GiB, used for testing

func init() {
	testing.AddTest(&testing.Test{
		Func: AutomaticCleanup,
		Desc: "Test automatic disk cleanup",
		Contacts: []string{
			"vsavu@google.com",     // Test author
			"gwendal@chromium.com", // Lead for Chrome OS Storage
		},
		Attr:    []string{"group:mainline", "informational"},
		Timeout: 5 * time.Minute,
	})
}

func AutomaticCleanup(ctx context.Context, s *testing.State) {
	const (
		userHome = "/home/user"

		mib               uint64 = 1024 * 1024              // 1 MiB
		gib               uint64 = 1024 * mib               // 1 GiB
		minimalFreeSpace         = 512 * mib                // hard-coded in cryptohomed
		cleanupTrigger           = 2 * gib                  // hard-coded in cryptohomed
		homedirSize              = 900 * mib                // 900 Mib, used for testing
		startingFreeSpace        = cleanupTrigger + 200*mib // 2.2 GiB, used for testing

		user1    = "cleanup-user1"
		user2    = "cleanup-user2"
		password = "1234"
	)

	createUserHomeAndFill := func(ctx context.Context, user, pass, fillDirectory, fillRoot string, clearedDirectories []string, size uint64, extraSetup func(userHash string) error) error {
		if err := cryptohome.CreateVault(ctx, user, pass); err != nil {
			return errors.Wrap(err, "failed to create user vault")
		}
		ok := false
		defer func() {
			if !ok {
				cryptohome.RemoveVault(ctx, user)
			}
		}()

		hash, err := cryptohome.UserHash(ctx, user)
		if err != nil {
			return errors.Wrap(err, "failed to get user hash")
		}

		waitReady := func(dir string) error {
			return testing.Poll(ctx, func(ctx context.Context) error {
				if _, err := os.Stat(filepath.Join(userHome, hash, dir)); err != nil {
					return err
				}
				return nil
			}, &testing.PollOptions{Timeout: 10 * time.Second})
		}

		// Create a small file to check later if cleared
		for _, dir := range clearedDirectories {
			if err := waitReady(dir); err != nil {
				return err
			}

			_, err = disk.Fill(filepath.Join(userHome, hash, dir), 10)
			if err != nil {
				return errors.Wrap(err, "failed to fill space")
			}
		}

		if fillRoot == userHome {
			if err := waitReady(fillDirectory); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Join(fillRoot, hash, fillDirectory), os.ModePerm); err != nil {
				return errors.Wrap(err, "failed to create fill directory")
			}
		}

		_, err = disk.Fill(filepath.Join(fillRoot, hash, fillDirectory), size)
		if err != nil {
			return errors.Wrap(err, "failed to fill space")
		}

		if extraSetup != nil {
			if err := extraSetup(hash); err != nil {
				return errors.Wrap(err, "failed extraSetup")
			}
		}

		ok = true
		return nil
	}

	checkAutomaticCleanup := func(ctx context.Context, s *testing.State, fillDirectory, fillRoot string, clearedDirectories []string, threshold uint64, extraSetup func(userHash string) error) {
		// By using homedirSize bytes for each homedir we will be 100 MiB bellow the threshold
		homedirSize := (startingFreeSpace-threshold)/2 + 50*mib

		if err := upstart.EnsureJobRunning(ctx, "cryptohomed"); err != nil {
			s.Fatal("Failed to start cryptohomed: ", err)
		}

		// Wait for DBus to be available
		if err := cryptohome.CheckService(ctx); err != nil {
			s.Fatal("Failed to start cryptohomed: ", err)
		}

		// Stay above trigger for cleanup
		fillFile, err := disk.FillUntil(userHome, startingFreeSpace)
		if err != nil {
			s.Fatal("Failed to fill space: ", err)
		}
		defer os.Remove(fillFile.Name())
		defer fillFile.Close()

		freeSpace, err := disk.FreeSpace(userHome)
		if err != nil {
			s.Fatal("Failed get free space: ", err)
		}

		// Sanity check
		if freeSpace < 2*homedirSize {
			s.Fatal("Too little free space is available: ", freeSpace)
		}

		s.Logf("%v bytes remaining", freeSpace)

		// Create users with contents to fill up disk space
		if err := createUserHomeAndFill(ctx, user1, password, fillDirectory, fillRoot, clearedDirectories, homedirSize, extraSetup); err != nil {
			s.Fatal("Failed to create user with content: ", err)
		}
		defer cryptohome.RemoveVault(ctx, user1)

		if err := createUserHomeAndFill(ctx, user2, password, fillDirectory, fillRoot, clearedDirectories, homedirSize, extraSetup); err != nil {
			s.Fatal("Failed to create user with content: ", err)
		}
		defer cryptohome.RemoveVault(ctx, user2)

		// Make sure disk space is low
		freeSpace, err = disk.FreeSpace(userHome)
		if err != nil {
			s.Fatal("Failed get free space: ", err)
		}

		if freeSpace > threshold {
			s.Errorf("Space was not filled, %v available", freeSpace)
		}

		// Unmount just the first user
		if err := cryptohome.UnmountVault(ctx, user1); err != nil {
			s.Fatal("Failed to unmount user vault: ", err)
		}

		if err := cryptohome.CreateVault(ctx, user2, password); err != nil {
			s.Fatal("Failed to remount user vault: ", err)
		}

		if err := cryptohome.WaitForUserMount(ctx, user2); err != nil {
			s.Fatal("Failed to remount user vault: ", err)
		}

		// Keep file reference to prevent unmount on restart
		hash, err := cryptohome.UserHash(ctx, user2)
		if err != nil {
			s.Fatal("Failed to get user hash: ", err)
		}

		files, err := ioutil.ReadDir(filepath.Join(userHome, hash, "Cache"))
		if err != nil {
			s.Fatal("Failed to read directory: ", err)
		}

		if len(files) == 0 {
			s.Fatalf("Directory %s is empty", filepath.Join(userHome, hash, "Cache"))
		}

		file, err := os.Open(filepath.Join(userHome, hash, "Cache", files[0].Name()))
		if err != nil {
			s.Fatal("Failed to open file: ", err)
		}
		defer cryptohome.UnmountVault(ctx, user2)
		defer file.Close()

		// Restart to trigger cleanup
		if err := upstart.RestartJob(ctx, "cryptohomed"); err != nil {
			s.Fatal("Failed to restart cryptohomed: ", err)
		}

		s.Log("Waiting for cleanup to start")
		spaceBeforeCleanup := freeSpace
		// Wait for cleanup to start
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			freeSpace, err := disk.FreeSpace(userHome)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get free space"))
			}

			if freeSpace > spaceBeforeCleanup {
				return nil
			}

			return errors.Errorf("too little disk space %v", freeSpace)
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			s.Error("Space was not cleared: ", err)
		}

		// Wait for cleanup to finish
		testing.Sleep(ctx, 2*time.Second)

		freeSpace, err = disk.FreeSpace(userHome)
		if err != nil {
			s.Fatal("Failed get free space: ", err)
		}

		if freeSpace > threshold+homedirSize {
			s.Errorf("Mounted user was cleaned up, %v free space available", freeSpace)
		}
	}

	for _, param := range []struct {
		name               string
		threshold          uint64
		fillRoot           string
		fillDirectory      string
		clearedDirectories []string // not based on fillRoot
		extraSetup         func(userHash string) error
	}{
		{
			name:               "cache",
			threshold:          gib,
			fillRoot:           userHome,
			fillDirectory:      "Cache",
			clearedDirectories: []string{},
		}, {
			name:               "gcache-v1",
			threshold:          gib,
			fillRoot:           userHome,
			fillDirectory:      "GCache/v1/tmp",
			clearedDirectories: []string{"Cache"},
		},
		{
			name:               "gcache-v2",
			threshold:          gib,
			fillRoot:           userHome,
			fillDirectory:      "GCache/v2/",
			clearedDirectories: []string{"Cache"},
			extraSetup: func(userHash string) error {
				fillDirectory := filepath.Join(userHome, userHash, "GCache/v2/")

				files, err := ioutil.ReadDir(fillDirectory)
				if err != nil {
					return errors.Wrap(err, "failed to read filled directory")
				}

				if len(files) == 0 {
					return errors.Wrapf(err, "directory %s empty", fillDirectory)
				}

				filePath := filepath.Join(fillDirectory, files[0].Name())

				if err := xattr.Set(filePath, "user.GCacheRemovable", []byte("something")); err != nil {
					return errors.Wrap(err, "failed to set xattr")
				}

				return nil
			},
		}, {
			name:               "android",
			threshold:          minimalFreeSpace,
			fillRoot:           rootHome,
			fillDirectory:      "dir/package/cache",
			clearedDirectories: []string{"Cache", "GCache/v1/tmp"},
			extraSetup: func(userHash string) error {
				packagePath := filepath.Join(rootHome, userHash, "dir/package/")
				cachePath := filepath.Join(packagePath, "cache")

				cacheInfo, err := os.Stat(cachePath)
				if err != nil {
					return errors.Wrapf(err, "failed to stat fill file %s", cachePath)
				}

				stat, ok := cacheInfo.Sys().(*syscall.Stat_t)
				if !ok {
					return errors.Wrapf(err, "failed to get inode of file %s", cachePath)
				}

				cacheInode := stat.Ino

				b := make([]byte, 8)
				binary.LittleEndian.PutUint64(b, cacheInode)

				if err := xattr.Set(packagePath, "user.inode_cache", b); err != nil {
					return errors.Wrap(err, "failed to set xattr")
				}

				return nil
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			checkAutomaticCleanup(ctx, s, param.fillDirectory, param.fillRoot, param.clearedDirectories, param.threshold, param.extraSetup)
		})
	}
}
