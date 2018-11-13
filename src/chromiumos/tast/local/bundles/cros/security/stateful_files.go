// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	chk "chromiumos/tast/local/bundles/cros/security/filecheck"
	"chromiumos/tast/local/bundles/cros/security/filesetup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StatefulFiles,
		Desc: "Checks ownership and permissions of files on the stateful partition",
		Attr: []string{"informational"},
	})
}

func StatefulFiles(ctx context.Context, s *testing.State) {
	const (
		root      = "/mnt/stateful_partition"
		errorFile = "errors.txt"
		maxErrors = 5 // max to print
	)

	// users returns an option that permits a path to be owned by any of the supplied users (all of which must exist).
	users := func(usernames ...string) chk.Option {
		uids := make([]int, len(usernames))
		for i, u := range usernames {
			uids[i] = filesetup.GetUID(u)
		}
		return chk.UID(uids...)
	}
	group := func(g string) chk.Option { return chk.GID(filesetup.GetGID(g)) }

	// The basic approach here is to specify patterns for paths within a top-level directory, and then add a catch-all
	// Tree pattern that checks anything in the directory that wasn't already explicitly checked or skipped.
	// Any top-level directories not explicitly handled are ignored.
	patterns := []*chk.Pattern{
		chk.NewPattern(chk.Path("dev_image"), chk.SkipChildren()), // only exists for dev images

		chk.NewPattern(chk.Path("encrypted/chronos"), users("chronos"), group("chronos"), chk.Mode(0755), chk.SkipChildren()), // contents checked by security.UserFiles*
		chk.NewPattern(chk.Path("encrypted/var/cache/app_pack"), users("chronos"), group("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_local_account_component_policy"), users("chronos"), group("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_local_account_extensions"), users("chronos"), group("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_local_account_external_policy_data"), users("chronos"), group("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_policy_external_data"), users("chronos"), group("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/display_profiles"), users("chronos"), group("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/edb"), users("root"), group("portage"), chk.Mode(0755), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache/echo"), users("root"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/cache/external_cache"), users("chronos"), group("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache/ldconfig"), users("root"), group("root"), chk.NotMode(077)),
		chk.NewPattern(chk.Path("encrypted/var/cache/shared_extensions"), users("chronos"), group("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache/shill"), users("shill"), group("shill"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/cache/signin_profile_component_policy"), users("chronos"), group("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache"), users("root"), group("root"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/coredumps"), users("chronos"), group("chronos"), chk.NotMode(077)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/bluetooth"), users("bluetooth"), chk.NotMode(027)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/chaps"), users("chaps"), group("chronos-access"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/chaps/database"), users("chaps"), group("chronos-access"), chk.NotMode(027)),
		chk.NewPattern(chk.Path("encrypted/var/lib/cromo"), users("root"), group("root"), chk.Mode(0755)), // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/lib/cromo"), users("chronos", "root"), chk.NotMode(022)),   // children
		chk.NewPattern(chk.Tree("encrypted/var/lib/dhcpcd"), users("dhcp"), group("dhcp"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/lib/gentoo"), users("root"), chk.NotMode(022), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/lib/imageloader"), users("imageloaderd"), group("imageloaderd"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/lib/metrics/uma-events"), users("chronos"), group("chronos"), chk.Mode(0666)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/power_manager"), users("power"), group("power"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/shill"), users("shill"), group("shill"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/timezone"), users("chronos", "root"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/tpm"), users("root"), group("root"), chk.NotMode(077)),
		chk.NewPattern(chk.Path("encrypted/var/lib/whitelist"), users("root"), group("policy-readers"), chk.Mode(0750)), // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/lib/whitelist"), users("root"), group("root"), chk.Mode(0604)),           // children
		chk.NewPattern(chk.Tree("encrypted/var/lib"), users("root"), group("root"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/log/asan"), users("root"), group("root"), chk.Mode(0777|os.ModeSticky)),
		chk.NewPattern(chk.Tree("encrypted/var/log/chrome/Crash Reports"), users("chronos"), group("chronos"), chk.NotMode(077)),
		chk.NewPattern(chk.Tree("encrypted/var/log/chrome"), users("chronos"), group("chronos"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/log/emerge.log"), users("portage"), group("portage"), chk.Mode(0660)),
		chk.NewPattern(chk.Tree("encrypted/var/log/metrics"), users("root", "chronos", "shill"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/log/power_manager"), users("power"), group("power"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/log"), users("root"), group("syslog"), chk.Mode(0775|os.ModeSticky)), // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/log"), users("syslog", "root"), group("root"), chk.NotMode(022)),     // children
		chk.NewPattern(chk.Path("encrypted/var/tmp"), users("root"), group("root"), chk.Mode(0777|os.ModeSticky), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted"), users("root"), chk.NotMode(022)),

		chk.NewPattern(chk.PathRegexp(`^encrypted\.`), users("root"), group("root"), chk.Mode(0600)),

		chk.NewPattern(chk.Tree("etc"), users("root"), chk.NotMode(022)),

		chk.NewPattern(chk.Path("home/.shadow"), users("root"), group("root"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("home/root"), users("root"), group("root"), chk.Mode(0771|os.ModeSticky)),                    // directory itself
		chk.NewPattern(chk.Tree("home/root"), users("root"), group("root"), chk.Mode(0700), chk.SkipChildren()),              // top-level children
		chk.NewPattern(chk.Path("home/user"), users("root"), group("root"), chk.Mode(0755)),                                  // directory itself
		chk.NewPattern(chk.Tree("home/user"), users("chronos"), group("chronos-access"), chk.Mode(0700), chk.SkipChildren()), // top-level children
		chk.NewPattern(chk.Tree("home"), users("root"), group("root"), chk.NotMode(022)),

		chk.NewPattern(chk.Tree("unencrypted/attestation"), users("attestation", "root"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("unencrypted/preserve"), users("root"), chk.NotMode(02)),                 // directory itself
		chk.NewPattern(chk.Path("unencrypted/preserve/log"), chk.SkipChildren()),                         // only exists for testing
		chk.NewPattern(chk.Tree("unencrypted/preserve"), users("attestation", "root"), chk.NotMode(022)), // other children
		chk.NewPattern(chk.Tree("unencrypted"), users("root"), chk.NotMode(022)),

		chk.NewPattern(chk.Path("var_overlay"), chk.SkipChildren()), // only exists for dev images

		chk.NewPattern(chk.Root(), users("root"), group("root"), chk.Mode(0755)), // stateful_partition directory itself
		chk.NewPattern(chk.AllPaths(), users("root"), chk.NotMode(022)),          // everything else not already matched
	}

	if _, err := user.Lookup("tss"); err == nil {
		patterns = append([]*chk.Pattern{
			chk.NewPattern(chk.Tree("var-overlay/lib/tpm"), users("tss"), chk.NotMode(022)),
		}, patterns...)
	}

	if _, err := user.Lookup("tpm_manager"); err == nil {
		patterns = append([]*chk.Pattern{
			chk.NewPattern(chk.Path("encrypted/var/lib/tpm_manager"), users("tpm_manager"), group("tpm_manager"), chk.NotMode(022)),
			chk.NewPattern(chk.Path("encrypted/var/lib/tpm_manager/local_tpm_data"), users("root"), group("root"), chk.NotMode(077)),
		}, patterns...)
	}

	if _, err := user.Lookup("android-root"); err == nil {
		patterns = append([]*chk.Pattern{
			chk.NewPattern(chk.Path("encrypted/var/cache/camera"), users("chronos", "root"), chk.NotMode(02), chk.SkipChildren()),
			chk.NewPattern(chk.Path("encrypted/var/lib/oemcrypto"), users("arc-oemcrypto"), group("arc-oemcrypto"), chk.Mode(0700), chk.SkipChildren()),
			chk.NewPattern(chk.Path("unencrypted/apkcache"), chk.Mode(0700), chk.SkipChildren()),
			chk.NewPattern(chk.Tree("unencrypted/art-data"), users("android-root", "root"), chk.NotMode(022)),
		}, patterns...)
	}

	s.Log("Checking ", root)
	problems, numPaths, err := chk.Check(ctx, root, patterns)
	s.Logf("Scanned %d path(s)", numPaths)
	if err != nil {
		s.Errorf("Failed to check %v: %v", root, err)
	}

	f, err := os.Create(filepath.Join(s.OutDir(), errorFile))
	if err != nil {
		s.Error("Failed to create error file: ", err)
	} else {
		defer f.Close()
		for path, msgs := range problems {
			if _, err := fmt.Fprintf(f, "%v: %v\n", path, strings.Join(msgs, ", ")); err != nil {
				s.Error("Failed to write error file: ", err)
				break
			}
		}
	}

	numErrors := 0
	for path, msgs := range problems {
		numErrors++
		if numErrors > maxErrors {
			s.Logf("Too many errors; aborting (see %v)", errorFile)
			break
		}
		s.Errorf("%v: %v", path, strings.Join(msgs, ", "))
	}
}
