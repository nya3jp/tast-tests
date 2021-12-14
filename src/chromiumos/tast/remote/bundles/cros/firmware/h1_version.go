// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     H1Version,
		Desc:     "Verifies that H1 is running either the prod or PVT version",
		Contacts: []string{"jbettis@chromium.org", "cros-fw-engprod@google.com"},
		Attr:     []string{"group:firmware", "firmware_bringup"},
		Vars:     []string{"servo"},
	})
}

// H1Version opens the H1 (cr50) console and verifies the version.
// Only runs from a chroot after running `sudo emerge chromeos-cr50`.
func H1Version(ctx context.Context, s *testing.State) {
	servoSpec, _ := s.Var("servo")
	h := firmware.NewHelperWithoutDUT("", servoSpec, s.DUT().KeyFile(), s.DUT().KeyDir())
	defer h.Close(ctx)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to require servo: ", err)
	}

	version, err := h.Servo.GetString(ctx, "cr50_version")
	if err != nil {
		s.Fatal("Failed to get cr50_version: ", err)
	}
	i := strings.Index(version, "/")
	shortVersion := version
	if i >= 0 {
		shortVersion = version[:i]
	}

	h1ver, err := extractH1ImageVersion(ctx, "/opt/google/cr50/firmware/cr50.bin.prod")
	if err != nil {
		s.Fatal("Failed to get prod version: ", err)
	}
	if h1ver == shortVersion {
		s.Log("H1 version matches cr50.bin.prod version: ", version)
		return
	}

	h1ver, err = extractH1ImageVersion(ctx, "/opt/google/cr50/firmware/cr50.bin.prepvt")
	if err != nil {
		s.Fatal("Failed to get prod version: ", err)
	}
	if h1ver == shortVersion {
		s.Log("H1 version matches cr50.bin.prepvt version: ", version)
		return
	}
	s.Fatalf("H1 version is incorrect: %s", shortVersion)
}

func extractH1ImageVersion(ctx context.Context, filename string) (string, error) {
	if _, err := os.Stat(filename); err != nil {
		return "", errors.Wrap(err, "please run sudo emerge chromeos-cr50")
	}
	output, err := exec.CommandContext(ctx, "/usr/sbin/gsctool", "-b", filename).Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get version from %s", filename)
	}
	versionRe := regexp.MustCompile(`RW_A:([0-9\.]+)`)
	matches := versionRe.FindSubmatch(output)
	if matches == nil {
		return "", errors.Errorf("failed to find RW_A version in %s", output)
	}
	return string(matches[1]), nil
}
