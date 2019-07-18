// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package device contains device-related test logic shared by audio tests.
package device

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// TestDeviceFiles tests device files matching pattern, a regular expression
// that must match a device node name (e.g. "^pcm.*$"), exist in /dev/snd
// with correct permissions.
func TestDeviceFiles(ctx context.Context, s *testing.State, pattern string) {
	const (
		dir  = "/dev/snd"
		mode = 0660
	)

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		s.Fatal("Failed to list files at /dev/snd: ", err)
	}

	if f, err := os.Create(filepath.Join(s.OutDir(), "ls.txt")); err != nil {
		s.Error("Failed to open output file: ", err)
	} else {
		defer f.Close()
		cmd := testexec.CommandContext(ctx, "ls", "-l", dir)
		cmd.Stdout = f
		cmd.Stderr = f
		if err := cmd.Run(); err != nil {
			s.Errorf("Failed to run ls on %v: %v", dir, err)
		}
	}

	check := func(ps string) {
		p := regexp.MustCompile(ps)
		found := false
		for _, fi := range files {
			if p.MatchString(fi.Name()) {
				if fi.Mode()&0777 != mode {
					s.Errorf("%s: permission mismatch: expected %o, actually %o", fi.Name(), mode, fi.Mode())
				}
				found = true
			}
		}
		if !found {
			s.Errorf("No file matched %s", ps)
		}
	}

	check(`^controlC\d+$`)
	check(pattern)
}

// TestALSACommand tests ALSA command recognizes devices.
func TestALSACommand(ctx context.Context, s *testing.State, name string) {
	out, err := testexec.CommandContext(ctx, name, "-l").CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", name, err)
	}
	if strings.Contains(string(out), "no soundcards found") {
		s.Errorf("%s recognized no sound cards", name)
	}
}
