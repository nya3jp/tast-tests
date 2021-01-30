// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"strings"

	chk "chromiumos/tast/local/bundles/cros/security/filecheck"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RunFiles,
		Desc: "Checks ownership and permissions of files in /run",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"yusukes@chromium.org", // Initial author
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func RunFiles(ctx context.Context, s *testing.State) {
	const (
		root = "/run"
	)
	patterns := []*chk.Pattern{
		// ARC/ARCVM files (crbug.com/1163122)
		chk.NewPattern(chk.PathRegexp("arc(vm)?/host_generated/.*\\.prop"), chk.UID(0), chk.GID(0), chk.Mode(0644)),
		// ARCVM-specific files (ignored on ARC builds)
		chk.NewPattern(chk.Path("arcvm/host_generated/fstab"), chk.UID(0), chk.GID(0), chk.Mode(0644)),
		chk.NewPattern(chk.Path("arcvm/host_generated/oem/etc/media_profiles.xml"), chk.Users("arc-camera"), chk.Groups("arc-camera"), chk.Mode(0644)),
		chk.NewPattern(chk.Path("arcvm/host_generated/oem/etc/permissions/platform.xml"), chk.Users("crosvm"), chk.Groups("crosvm"), chk.Mode(0644)),
	}

	problems, _, err := chk.Check(ctx, root, patterns)
	if err != nil {
		s.Errorf("Failed to check %v: %v", root, err)
	}
	for path, msgs := range problems {
		s.Errorf("%v: %v", path, strings.Join(msgs, ", "))
	}
}
