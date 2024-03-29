// Copyright 2018 The ChromiumOS Authors
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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	playbackDevicePath = "/proc/asound/card*/pcm*p/sub0/status"
	captureDevicePath  = "/proc/asound/card*/pcm*c/sub0/status"
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
// We want to check internal sound cards, ignore external devices as workaround.
func TestALSACommand(ctx context.Context, name string) error {
	out, err := testexec.CommandContext(ctx, name, "-l").CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		return err
	}
	if strings.Contains(string(out), "no soundcards found") {
		return errors.Errorf("%q recognized no sound cards", name)
	}
	//Ignore external devices.
	found := false
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "card") {
			found = found || IsInternalCard(line)
		}
	}
	if !found {
		return errors.Errorf("%q recognized no internal sound cards", name)
	}
	return nil
}

// IsInternalCard checks if the given ALSA command output is about an internal
// sound card.
func IsInternalCard(card string) bool {
	//Based on ALSA command output.
	externalCards := []string{"USB Audio", "HDMI"}
	for _, externalCard := range externalCards {
		if strings.Contains(card, externalCard) {
			return false
		}
	}
	return true
}

func findRunningDevice(ctx context.Context, pathPattern string) error {
	paths, err := filepath.Glob(pathPattern)
	if err != nil {
		return err
	}
	for _, p := range paths {
		b, err := ioutil.ReadFile(p)
		if err != nil {
			return errors.Wrapf(err, "failed to read %q", p)
		}

		s := string(b)
		if strings.Contains(s, "RUNNING") {
			return nil
		}
	}

	return errors.New("failed to find running device")
}

// CheckRunningDevice checks the running output/input device by parsing asound status.
// A device may not be opened immediately so it will repeat the query until the
// expected running device(s) are found.
func CheckRunningDevice(ctx context.Context, playback, capture bool) error {
	if !playback && !capture {
		return errors.New("at least one of playback and capture should be true")
	}
	if playback {
		if err := findRunningDevice(ctx, playbackDevicePath); err != nil {
			return errors.Errorf("failed to grep playback asound status: %s", err)
		}
	}

	if capture {
		if err := findRunningDevice(ctx, captureDevicePath); err != nil {
			return errors.Errorf("failed to grep capture asound status: %s", err)
		}
	}

	return nil
}
