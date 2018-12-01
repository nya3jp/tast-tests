// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"chromiumos/tast/local/bundles/cros/security/filesetup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StatefulPartitionHardening,
		Desc: "Tests access behavior of symlinks and FIFOs",
		Attr: []string{"informational"},
	})
}

func StatefulPartitionHardening(ctx context.Context, s *testing.State) {
	rand.Seed(time.Now().UnixNano())
	generateTempFilename := func(parent string) string {
		randBytes := make([]byte, 8)
		for i := 0; i < len(randBytes); i++ {
			// Generate random uppercase letters
			randBytes[i] = byte(65 + rand.Intn(26))
		}
		return filepath.Join(parent, "tast.security.StatefulPartitionHardening."+string(randBytes[:]))
	}

	expectAccess := func(path string, expected bool) {
		fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NONBLOCK, 0777)
		success := (err == nil)
		if success {
			syscall.Close(fd)
		}
		if success != expected {
			message := "succeeded"
			if !success {
				message = "failed"
			}
			s.Errorf("Attempt to open %v unexpectedly %v", path, message)
		}
	}

	expectFifoAccess := func(parent string, expected bool) {
		path := generateTempFilename(parent)
		if err := syscall.Mkfifo(path, 0666); err != nil {
			s.Fatalf("Failed to create fifo at %v: %v", path, err.Error())
		}
		defer os.Remove(path)
		expectAccess(path, expected)
	}

	expectSymlinkAccess := func(parent string, expected bool) {
		path := generateTempFilename(parent)
		filesetup.CreateSymlink("/dev/null", path, filesetup.GetUID("root"))
		defer os.Remove(path)
		expectAccess(path, expected)
		path = generateTempFilename(parent)
		filesetup.CreateSymlink("/dev", path, filesetup.GetUID("root"))
		defer os.Remove(path)
		expectAccess(filepath.Join(path, "null"), expected)
	}

	var blockedLocations = []string{
		"/mnt/stateful_partition",
		"/var",
	}

	var allowedLocations = []string{
		"/tmp",
		"/home",
	}

	var symlinkExceptions = []string{
		"/var/cache/echo",
		"/var/cache/vpd",
		"/var/lib/timezone",
		"/var/log",
	}

	// We also need to make sure that the restrictions apply to subdirectories, unless they are
	// treated specially (like the symlinkExceptions).
	makeSubdirs := func(locs []string) []string {
		tempLocs := make([]string, len(locs))
		for i, loc := range locs {
			path, err := ioutil.TempDir(loc, "tast.security.StatefulPartitionHardening.")
			if err != nil {
				s.Fatalf("Failed to create temp directory in %v", loc)
			}
			tempLocs[i] = path
		}
		return tempLocs
	}

	blockedTemps := makeSubdirs(blockedLocations)
	blockedLocations = append(blockedLocations, blockedTemps...)
	allowedTemps := makeSubdirs(allowedLocations)
	allowedLocations = append(allowedLocations, allowedTemps...)
	symlinkTemps := makeSubdirs(symlinkExceptions)
	symlinkExceptions = append(symlinkExceptions, symlinkTemps...)

	tempDirs := append(blockedTemps, allowedTemps...)
	tempDirs = append(tempDirs, symlinkTemps...)
	for _, td := range tempDirs {
		defer os.RemoveAll(td)
	}

	for _, loc := range blockedLocations {
		s.Logf("Checking that symlinks and fifos are blocked in %v", loc)
		expectSymlinkAccess(loc, false)
		expectFifoAccess(loc, false)
	}

	for _, loc := range allowedLocations {
		s.Logf("Checking that symlinks and fifos are allowed in %v", loc)
		expectSymlinkAccess(loc, true)
		expectFifoAccess(loc, true)
	}

	for _, loc := range symlinkExceptions {
		s.Logf("Checking that symlinks but not fifos are allowed in %v", loc)
		expectSymlinkAccess(loc, true)
		expectFifoAccess(loc, false)
	}
}
