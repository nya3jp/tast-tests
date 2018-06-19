// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package device

import (
	"io/ioutil"
	"regexp"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// TestDeviceFiles tests device files matching pattern, a regular expression
// that must match a device node name (e.g. "^pcm.*$"), exist in /dev/snd
// with correct permissions.
func TestDeviceFiles(s *testing.State, pattern string) {
	const mode = 0660

	files, err := ioutil.ReadDir("/dev/snd")
	if err != nil {
		s.Fatal("Failed to list files at /dev/snd: ", err)
	}

	check := func(ps string) {
		p := regexp.MustCompile(ps)
		found := false
		for _, f := range files {
			if p.MatchString(f.Name()) {
				if f.Mode()&0777 != mode {
					s.Errorf("%s: permission mismatch: expected %o, actually %o", f.Name(), mode, f.Mode())
				}
				found = true
			}
		}
		if !found {
			s.Errorf("no file matched %s", ps)
		}
	}

	check(`^controlC\d+$`)
	check(pattern)
}

// TestALSACommand tests ALSA command recognizes devices.
func TestALSACommand(s *testing.State, name string) {
	cmd := testexec.CommandContext(s.Context(), name, "-l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(s.Context())
		s.Errorf("%s failed: %v", name, err)
	}
	if strings.Contains(string(out), "no soundcards found") {
		s.Errorf("%s recognized no sound cards", name)
	}
}
