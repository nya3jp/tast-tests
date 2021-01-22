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
	"chromiumos/tast/local/moblab"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StatefulFiles,
		Desc: "Checks ownership and permissions of files on the stateful partition",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func StatefulFiles(ctx context.Context, s *testing.State) {
	const (
		root      = "/mnt/stateful_partition"
		errorFile = "errors.txt"
		maxErrors = 5 // max to print
	)

	// The basic approach here is to specify patterns for paths within a top-level directory, and then add a catch-all
	// Tree pattern that checks anything in the directory that wasn't already explicitly checked or skipped.
	// Any top-level directories not explicitly handled are matched by the final AllPaths pattern.
	patterns := []*chk.Pattern{
		chk.NewPattern(chk.Path("dev_image"), chk.SkipChildren()),     // only exists for dev images
		chk.NewPattern(chk.Path("dev_image_old"), chk.SkipChildren()), // only exists for dev images
		chk.NewPattern(chk.Path("dev_image_new"), chk.SkipChildren()), // only exists for dev images

		chk.NewPattern(chk.Path("encrypted/chronos"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0755), chk.SkipChildren()), // contents checked by security.UserFiles*

		chk.NewPattern(chk.Path("encrypted/var/cache/app_pack"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0700), chk.SkipChildren()),
		// TODO(crbug.com/905719): Check for a specific user:group and mode.
		chk.NewPattern(chk.Path("encrypted/var/cache/camera"), chk.Users(s, "chronos", "root"), chk.NotMode(02), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_local_account_component_policy"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_local_account_extensions"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_local_account_external_policy_data"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_policy_external_data"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/display_profiles"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/edb"), chk.Users(s, "root"), chk.Groups(s, "portage"), chk.Mode(0755), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache/echo"), chk.Users(s, "root"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/cache/external_cache"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache/ldconfig"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.NotMode(077)),
		chk.NewPattern(chk.Tree("encrypted/var/cache/modemfwd"), chk.Users(s, "modem"), chk.Groups(s, "modem"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/cache/shared_extensions"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache/shill"), chk.Users(s, "shill"), chk.Groups(s, "shill"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/cache/signin_profile_component_policy"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/signin_profile_extensions"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.NotMode(022)),

		chk.NewPattern(chk.Tree("encrypted/var/coredumps"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.NotMode(077)),

		chk.NewPattern(chk.Tree("encrypted/var/lib/bluetooth"), chk.Users(s, "bluetooth"), chk.NotMode(027)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/bootlockbox"), chk.Users(s, "bootlockboxd"), chk.Groups(s, "bootlockboxd"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/chaps"), chk.Users(s, "chaps"), chk.Groups(s, "chronos-access"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/lib/cras"), chk.Users(s, "cras"), chk.Groups(s, "cras"), chk.Mode(0755)),                     // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/lib/cras"), chk.Users(s, "cras"), chk.Groups(s, "cras"), chk.Mode(0644), chk.SkipChildren()), // children
		chk.NewPattern(chk.Tree("encrypted/var/lib/chaps/database"), chk.Users(s, "chaps"), chk.Groups(s, "chronos-access"), chk.NotMode(027)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/dhcpcd"), chk.Users(s, "dhcp"), chk.Groups(s, "dhcp"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/lib/gentoo"), chk.Users(s, "root"), chk.NotMode(022), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/lib/imageloader"), chk.Users(s, "imageloaderd"), chk.Groups(s, "imageloaderd"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/lib/metrics/uma-events"), chk.Users(s, "metrics"), chk.Groups(s, "metrics"), chk.Mode(0666)),
		chk.NewPattern(chk.Path("encrypted/var/lib/metrics"), chk.Users(s, "metrics"), chk.Groups(s, "metrics"), chk.Mode(0755)),                                     // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/lib/metrics"), chk.Users(s, "metrics", "root"), chk.Groups(s, "metrics", "root"), chk.Mode(0644), chk.SkipChildren()), // children
		chk.NewPattern(chk.Tree("encrypted/var/lib/ml_service"), chk.Users(s, "ml-service"), chk.Groups(s, "ml-service"), chk.NotMode(02)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/modemfwd"), chk.Users(s, "modem"), chk.Groups(s, "modem"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/oobe_config_restore"), chk.Users(s, "oobe_config_restore"), chk.Groups(s, "oobe_config_restore"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/oobe_config_save"), chk.Users(s, "oobe_config_save"), chk.Groups(s, "oobe_config_save"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/power_manager"), chk.Users(s, "power"), chk.Groups(s, "power"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/shill"), chk.Users(s, "shill"), chk.Groups(s, "shill"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/timezone"), chk.Users(s, "chronos", "root"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/tpm"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.NotMode(077)),
		chk.NewPattern(chk.Path("encrypted/var/lib/whitelist"), chk.Users(s, "root"), chk.Groups(s, "policy-readers"), chk.Mode(0750)), // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/lib/whitelist"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.NotMode(022)),         // children
		chk.NewPattern(chk.Tree("encrypted/var/lib"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.NotMode(022)),

		chk.NewPattern(chk.Tree("encrypted/var/log/asan"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.Mode(0777|os.ModeSticky)),
		chk.NewPattern(chk.Tree("encrypted/var/log/chrome/Crash Reports/uploads.log"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.Mode(0644)),
		chk.NewPattern(chk.Tree("encrypted/var/log/chrome/Crash Reports"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.NotMode(077)),
		chk.NewPattern(chk.Tree("encrypted/var/log/chrome"), chk.Users(s, "chronos"), chk.Groups(s, "chronos"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/log/emerge.log"), chk.Users(s, "portage"), chk.Groups(s, "portage"), chk.Mode(0660)),
		chk.NewPattern(chk.Tree("encrypted/var/log/metrics"), chk.Users(s, "root", "chronos", "metrics", "shill"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/log/modemfwd"), chk.Users(s, "modem"), chk.Groups(s, "modem"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/log/power_manager"), chk.Users(s, "power"), chk.Groups(s, "power"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/log/usbmon"), chk.Users(s, "root", "tcpdump"), chk.Groups(s, "root", "tcpdump"), chk.SkipChildren()), // only created by tests
		chk.NewPattern(chk.Path("encrypted/var/log/vmlog"), chk.Users(s, "metrics"), chk.Groups(s, "metrics"), chk.Mode(0755)),                      // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/log/vmlog"), chk.Users(s, "metrics"), chk.Groups(s, "metrics"), chk.Mode(0644)),                      // children
		chk.NewPattern(chk.Path("encrypted/var/log"), chk.Users(s, "root"), chk.Groups(s, "syslog"), chk.Mode(0775|os.ModeSticky)),                  // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/log"), chk.Users(s, "syslog", "root"), chk.Groups(s, "syslog", "root"), chk.NotMode(022)),            // children

		chk.NewPattern(chk.Path("encrypted/var/spool/crash"), chk.Users(s, "root"), chk.Groups(s, "crash-access"), chk.Mode(0770|os.ModeSetgid)), // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/spool/crash"), chk.Users(s, "root"), chk.Groups(s, "crash-access"), chk.NotMode(002)),             // children

		chk.NewPattern(chk.Path("encrypted/var/tmp"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.Mode(0777|os.ModeSticky), chk.SkipChildren()),

		chk.NewPattern(chk.Tree("encrypted"), chk.Users(s, "root"), chk.NotMode(022)),
		chk.NewPattern(chk.PathRegexp(`^encrypted\.`), chk.Users(s, "root"), chk.Groups(s, "root"), chk.Mode(0600)),

		chk.NewPattern(chk.Tree("etc"), chk.Users(s, "root"), chk.NotMode(022)),

		chk.NewPattern(chk.Path("home/.shadow"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("home/root"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.Mode(0751|os.ModeSticky)),                    // directory itself
		chk.NewPattern(chk.Tree("home/root"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.Mode(0700), chk.SkipChildren()),              // top-level children
		chk.NewPattern(chk.Path("home/user"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.Mode(0755)),                                  // directory itself
		chk.NewPattern(chk.Tree("home/user"), chk.Users(s, "chronos"), chk.Groups(s, "chronos-access"), chk.Mode(0700), chk.SkipChildren()), // top-level children
		chk.NewPattern(chk.Tree("home"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.NotMode(022)),

		chk.NewPattern(chk.Path("unencrypted/apkcache"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("unencrypted/attestation"), chk.Users(s, "attestation", "root"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("unencrypted/preserve"), chk.Users(s, "root"), chk.NotMode(02)),                 // directory itself
		chk.NewPattern(chk.Path("unencrypted/preserve/cros-update"), chk.SkipChildren()),                        // only exists for testing
		chk.NewPattern(chk.Path("unencrypted/preserve/log"), chk.SkipChildren()),                                // only exists for testing
		chk.NewPattern(chk.Tree("unencrypted/preserve"), chk.Users(s, "attestation", "root"), chk.NotMode(022)), // other children
		chk.NewPattern(chk.Path("unencrypted/chk.Userspace_swap.tmp"), chk.Users(s, "chronos"), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("unencrypted"), chk.Users(s, "root"), chk.NotMode(022)),

		chk.NewPattern(chk.Path("var_overlay"), chk.SkipChildren()), // only exists for dev images

		// This file can be created by
		// https://source.corp.google.com/chromeos_public/src/platform/factory/py/gooftool/wipe.py.
		// TODO(crbug.com/1083285): Avoid creating this file with 0666 permissions.
		chk.NewPattern(chk.Path("wipe_mark_file"), chk.Mode(0666)),

		chk.NewPattern(chk.Root(), chk.Users(s, "root"), chk.Groups(s, "root"), chk.Mode(0755)), // stateful_partition directory itself
		chk.NewPattern(chk.AllPaths(), chk.Users(s, "root"), chk.NotMode(022)),                  // everything else not already matched
	}

	// prependPatterns prepends the supplied patterns to the main patterns slice.
	prependPatterns := func(newPatterns ...*chk.Pattern) { patterns = append(newPatterns, patterns...) }

	if _, err := user.Lookup("tss"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("var-overlay/lib/tpm"), chk.Users(s, "tss"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("tpm_manager"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Path("encrypted/var/lib/tpm_manager"), chk.Users(s, "tpm_manager"), chk.Groups(s, "tpm_manager"), chk.NotMode(022)),
			chk.NewPattern(chk.Path("encrypted/var/lib/tpm_manager/local_tpm_data"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.NotMode(077)))
	}

	if _, err := user.Lookup("trunks"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Path("encrypted/var/lib/trunks"), chk.Users(s, "trunks"), chk.Groups(s, "trunks"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("biod"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Tree("encrypted/var/log/bio_crypto_init"), chk.Users(s, "biod", "root"), chk.Groups(s, "biod", "root"), chk.NotMode(022)),
			chk.NewPattern(chk.Tree("encrypted/var/log/biod"), chk.Users(s, "biod", "root"), chk.Groups(s, "biod", "root"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("buffet"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/lib/buffet"), chk.Users(s, "buffet"), chk.Groups(s, "buffet"), chk.NotMode(02)))
	}

	if _, err := user.Lookup("cups"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Tree("encrypted/var/cache/cups"), chk.Users(s, "cups"), chk.Groups(s, "cups", "nobody"), chk.NotMode(02)),
			chk.NewPattern(chk.Tree("encrypted/var/spool/cups"), chk.Users(s, "cups"), chk.Groups(s, "cups", "nobody"), chk.NotMode(02)))
	}

	if _, err := user.Lookup("android-root"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Tree("unencrypted/art-data"), chk.Users(s, "android-root", "root"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("cdm-oemcrypto"); err == nil {
		prependPatterns(chk.NewPattern(chk.Path("encrypted/var/lib/oemcrypto"), chk.Users(s, "cdm-oemcrypto"), chk.Groups(s, "cdm-oemcrypto"), chk.Mode(0700), chk.SkipChildren()))
	}

	if _, err := user.Lookup("fwupd"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Path("encrypted/var/cache/fwupd"), chk.Users(s, "fwupd"), chk.Groups(s, "fwupd"), chk.SkipChildren()),
			chk.NewPattern(chk.Path("encrypted/var/lib/fwupd"), chk.Users(s, "fwupd"), chk.Groups(s, "fwupd"), chk.SkipChildren()))
	}

	if _, err := user.Lookup("dlcservice"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/cache/dlc"), chk.Users(s, "dlcservice"), chk.Groups(s, "dlcservice"), chk.NotMode(022)))
		// encrypted dlc-images is created by dev_utils.sh script prior to bind
		// mounting unencrypted dlc-images directory.
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/cache/dlc-images"), chk.Users(s, "root"), chk.Groups(s, "root"), chk.NotMode(022)))
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/lib/dlcservice"), chk.Users(s, "dlcservice"), chk.Groups(s, "dlcservice"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("wilco_dtc"); err == nil {
		prependPatterns(chk.NewPattern(chk.Path("encrypted/var/lib/wilco/storage.img"), chk.Users(s, "wilco_dtc"), chk.Groups(s, "wilco_dtc"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("cros_healthd"); err == nil {
		prependPatterns(chk.NewPattern(chk.Path("encrypted/var/cache/diagnostics"), chk.Users(s, "cros_healthd"), chk.Groups(s, "cros_healthd"), chk.SkipChildren()))
	}

	if _, err := user.Lookup("displaylink"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/log/displaylink"), chk.Users(s, "displaylink"), chk.Groups(s, "displaylink"), chk.NotMode(022), chk.SkipChildren()))
	}

	if _, err := user.Lookup("sound_card_init"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/lib/sound_card_init"), chk.Users(s, "sound_card_init"), chk.Groups(s, "sound_card_init"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("rmtfs"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/lib/rmtfs"), chk.Users(s, "rmtfs"), chk.NotMode(022)))
	}

	if moblab.IsMoblab() {
		// On moblab devices, there are additional user dirs and tons of stuff (MySQL, etc.) in /var.
		prependPatterns(
			chk.NewPattern(chk.Tree("home/chronos"), chk.Users(s, "chronos", "root")),
			chk.NewPattern(chk.Tree("home/moblab"), chk.Users(s, "moblab", "root")),
			chk.NewPattern(chk.Tree("var"), chk.SkipChildren()))
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
