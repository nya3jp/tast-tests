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
		Desc:     "Verifies that H1 is running either the prod or pre-PVT version",
		Contacts: []string{"jbettis@chromium.org", "cros-fw-engprod@google.com"},
		Attr:     []string{"group:firmware", "firmware_bringup"},
		Vars:     []string{"servo"},
	})
}

var versionRe = regexp.MustCompile(`RW_A:([0-9\.]+)`)

// H1Version opens the H1 (cr50) console and verifies the version.
// Only runs from a chroot after running `sudo emerge chromeos-cr50`.
func H1Version(ctx context.Context, s *testing.State) {
	h1verProd, err := extractH1ImageVersion(ctx, "/opt/google/cr50/firmware/cr50.bin.prod")
	if err != nil {
		s.Fatal("Failed to get prod version: ", err)
	}
	h1verPrepvt, err := extractH1ImageVersion(ctx, "/opt/google/cr50/firmware/cr50.bin.prepvt")
	if err != nil {
		s.Fatal("Failed to get prod version: ", err)
	}

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
	shortVersion := strings.SplitN(version, "/", 2)[0]

	if h1verProd == shortVersion {
		s.Logf("H1 version matches cr50.bin.prod got %q, want %q", version, h1verProd)
		return
	}

	if h1verPrepvt == shortVersion {
		s.Logf("H1 version matches cr50.bin.prepvt got %q, want %q", version, h1verPrepvt)
		return
	}
	s.Fatalf("H1 version is incorrect: got %q, want %q or %q", version, h1verProd, h1verPrepvt)
}

func extractH1ImageVersion(ctx context.Context, filename string) (string, error) {
	if _, err := os.Stat(filename); err != nil {
		return "", errors.Wrapf(err, "failed to find %q please run sudo emerge chromeos-cr50", filename)
	}
	output, err := exec.CommandContext(ctx, "/usr/sbin/gsctool", "-b", filename).Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get version from %s", filename)
	}
	matches := versionRe.FindSubmatch(output)
	if matches == nil {
		return "", errors.Errorf("failed to find RW_A version in %s", output)
	}
	return string(matches[1]), nil
}
