// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash interacts with on-device crash reports.
package crash

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultCrashDir contains the directory where the kernel writes core and minidump files.
	DefaultCrashDir = "/var/spool/crash"
	// ChromeCrashDir contains the directory where Chrome writes minidump files.
	ChromeCrashDir = "/home/chronos/crash"

	coreSuffix     = ".core" // suffix for core files
	minidumpSuffix = ".dmp"  // suffix for minidump files
)

// GetCrashes returns the paths of core and minidump files generated in response to crashes.
func GetCrashes(dir string) (cores, minidumps []string, err error) {
	cores = make([]string, 0)
	minidumps = make([]string, 0)

	wf := func(path string, info os.FileInfo, err error) error {
		if path == dir {
			return nil
		} else if info.IsDir() {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, coreSuffix) {
			cores = append(cores, path)
		} else if strings.HasSuffix(path, minidumpSuffix) {
			minidumps = append(minidumps, path)
		}
		return nil
	}
	err = filepath.Walk(dir, wf)
	return cores, minidumps, err
}

// CopyNewFiles copies paths that are present in newPaths but not in oldPaths into dstDir.
// If maxPerExec is positive, it limits the maximum number of files that will be copied
// for each base executable.
func CopyNewFiles(dstDir string, newPaths, oldPaths []string, maxPerExec int) (
	warnings map[string]error, err error) {
	oldMap := make(map[string]struct{})
	for _, p := range oldPaths {
		oldMap[p] = struct{}{}
	}

	warnings = make(map[string]error)
	execCount := make(map[string]int)
	for _, sp := range newPaths {
		if _, ok := oldMap[sp]; ok {
			continue
		}

		var base string
		if parts := strings.Split(filepath.Base(sp), "."); len(parts) > 2 {
			// If there are at least three components in the crash filename, assume
			// that it's something like name.id.dmp and count the first part.
			base = filepath.Join(filepath.Dir(sp), parts[0])
		} else {
			// Otherwise, add it to the per-directory count.
			base = filepath.Dir(sp)
		}
		if maxPerExec > 0 && execCount[base] == maxPerExec {
			warnings[sp] = errors.New("skipping; too many files")
			continue
		}

		sf, err := os.Open(sp)
		if err != nil {
			warnings[sp] = err
			continue
		}
		defer sf.Close()

		dp := filepath.Join(dstDir, filepath.Base(sp))
		df, err := os.Create(dp)
		if err != nil {
			warnings[sp] = err
			continue
		}
		defer df.Close()

		if _, err := io.Copy(df, sf); err != nil {
			warnings[sp] = err
			continue
		}
		execCount[base] += 1
	}
	return warnings, nil
}
