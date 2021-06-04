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

		chk.NewPattern(chk.Path("encrypted/chronos"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0755), chk.SkipChildren()), // contents checked by security.UserFiles*

		chk.NewPattern(chk.Path("encrypted/var/cache/app_pack"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0700), chk.SkipChildren()),
		// TODO(crbug.com/905719): Check for a specific user:group and mode.
		chk.NewPattern(chk.Path("encrypted/var/cache/camera"), chk.Users("chronos", "root"), chk.NotMode(02), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_local_account_component_policy"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_local_account_extensions"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_local_account_external_policy_data"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/device_policy_external_data"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/display_profiles"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/edb"), chk.Users("root"), chk.Groups("portage"), chk.Mode(0755), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache/echo"), chk.Users("root"), chk.NotMode(022)),
		// Temporary directory to create external_cache. Most of time, it's owned by root, but switched to chronos just before its renaming.
		// See also crx-import.sh for details.
		chk.NewPattern(chk.Path("encrypted/var/cache/external_cache.tmp"), chk.Users("chronos", "root"), chk.Groups("chronos", "root"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/external_cache"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache/ldconfig"), chk.Users("root"), chk.Groups("root"), chk.NotMode(077)),
		chk.NewPattern(chk.Tree("encrypted/var/cache/modemfwd"), chk.Users("modem"), chk.Groups("modem"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/cache/modem-utilities"), chk.Users("shill-scripts"), chk.Groups("shill-scripts"), chk.Mode(0664)),
		chk.NewPattern(chk.Path("encrypted/var/cache/shared_extensions"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache/shill"), chk.Users("shill"), chk.Groups("shill"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/cache/signin_profile_component_policy"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("encrypted/var/cache/signin_profile_extensions"), chk.Users("chronos"), chk.Groups("chronos"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/cache"), chk.Users("root"), chk.Groups("root"), chk.NotMode(022)),

		chk.NewPattern(chk.Tree("encrypted/var/coredumps"), chk.Users("chronos"), chk.Groups("chronos"), chk.NotMode(077)),

		chk.NewPattern(chk.Tree("encrypted/var/lib/bluetooth"), chk.Users("bluetooth"), chk.NotMode(027)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/bootlockbox"), chk.Users("bootlockboxd"), chk.Groups("bootlockboxd"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/chaps"), chk.Users("chaps"), chk.Groups("chronos-access"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/lib/cras"), chk.Users("cras"), chk.Groups("cras"), chk.Mode(0755)),                     // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/lib/cras"), chk.Users("cras"), chk.Groups("cras"), chk.Mode(0644), chk.SkipChildren()), // children
		chk.NewPattern(chk.Tree("encrypted/var/lib/chaps/database"), chk.Users("chaps"), chk.Groups("chronos-access"), chk.NotMode(027)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/dhcpcd"), chk.Users("dhcp"), chk.Groups("dhcp"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/lib/gentoo"), chk.Users("root"), chk.NotMode(022), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/lib/imageloader"), chk.Users("imageloaderd"), chk.Groups("imageloaderd"), chk.NotMode(022)),
		// TODO(chromium:1197973): Re-add permissions checks for /var/lib/metrics
		chk.NewPattern(chk.Tree("encrypted/var/lib/metrics"), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("encrypted/var/lib/ml_service"), chk.Users("ml-service"), chk.Groups("ml-service"), chk.NotMode(02)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/modemfwd"), chk.Users("modem"), chk.Groups("modem"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/oobe_config_restore"), chk.Users("oobe_config_restore"), chk.Groups("oobe_config_restore"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/oobe_config_save"), chk.Users("oobe_config_save"), chk.Groups("oobe_config_save"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/power_manager"), chk.Users("power"), chk.Groups("power"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/shill"), chk.Users("shill"), chk.Groups("shill"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/timezone"), chk.Users("chronos", "root"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/lib/tpm"), chk.Users("root"), chk.Groups("root"), chk.NotMode(077)),
		chk.NewPattern(chk.Path("encrypted/var/lib/whitelist"), chk.Users("root"), chk.Groups("policy-readers"), chk.Mode(0750)), // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/lib/whitelist"), chk.Users("root"), chk.Groups("root"), chk.NotMode(022)),         // children
		chk.NewPattern(chk.Tree("encrypted/var/lib"), chk.Users("root"), chk.Groups("root"), chk.NotMode(022)),

		chk.NewPattern(chk.Tree("encrypted/var/log/asan"), chk.Users("root"), chk.Groups("root"), chk.Mode(0777|os.ModeSticky)),
		chk.NewPattern(chk.Tree("encrypted/var/log/chrome/Crash Reports/uploads.log"), chk.Users("root"), chk.Groups("root"), chk.Mode(0644)),
		chk.NewPattern(chk.Tree("encrypted/var/log/chrome/Crash Reports"), chk.Users("chronos"), chk.Groups("chronos"), chk.NotMode(077)),
		chk.NewPattern(chk.Tree("encrypted/var/log/chrome"), chk.Users("chronos"), chk.Groups("chronos"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/log/emerge.log"), chk.Users("portage"), chk.Groups("portage"), chk.Mode(0660)),
		chk.NewPattern(chk.Tree("encrypted/var/log/metrics"), chk.Users("root", "chronos", "metrics", "shill"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/log/modemfwd"), chk.Users("modem"), chk.Groups("modem"), chk.NotMode(022)),
		chk.NewPattern(chk.Tree("encrypted/var/log/power_manager"), chk.Users("power"), chk.Groups("power"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("encrypted/var/log/usbmon"), chk.Users("root", "tcpdump"), chk.Groups("root", "tcpdump"), chk.SkipChildren()), // only created by tests
		chk.NewPattern(chk.Path("encrypted/var/log/vmlog"), chk.Users("metrics"), chk.Groups("metrics"), chk.Mode(0755)),                      // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/log/vmlog"), chk.Users("metrics"), chk.Groups("metrics"), chk.Mode(0644)),                      // children
		chk.NewPattern(chk.Path("encrypted/var/log"), chk.Users("root"), chk.Groups("syslog"), chk.Mode(0775|os.ModeSticky)),                  // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/log"), chk.Users("syslog", "root"), chk.Groups("syslog", "root"), chk.NotMode(022)),            // children

		chk.NewPattern(chk.Path("encrypted/var/spool/crash"), chk.Users("root"), chk.Groups("crash-access"), chk.Mode(0770|os.ModeSetgid)), // directory itself
		chk.NewPattern(chk.Tree("encrypted/var/spool/crash"), chk.Users("root"), chk.Groups("crash-access"), chk.NotMode(002)),             // children

		chk.NewPattern(chk.Path("encrypted/var/tmp"), chk.Users("root"), chk.Groups("root"), chk.Mode(0777|os.ModeSticky), chk.SkipChildren()),

		chk.NewPattern(chk.Tree("encrypted"), chk.Users("root"), chk.NotMode(022)),
		chk.NewPattern(chk.PathRegexp(`^encrypted\.`), chk.Users("root"), chk.Groups("root"), chk.Mode(0600)),

		chk.NewPattern(chk.Tree("etc"), chk.Users("root"), chk.NotMode(022)),

		chk.NewPattern(chk.Path("home/.shadow"), chk.Users("root"), chk.Groups("root"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Path("home/root"), chk.Users("root"), chk.Groups("root"), chk.Mode(0751|os.ModeSticky)),                    // directory itself
		chk.NewPattern(chk.Tree("home/root"), chk.Users("root"), chk.Groups("root"), chk.Mode(0700), chk.SkipChildren()),              // top-level children
		chk.NewPattern(chk.Path("home/user"), chk.Users("root"), chk.Groups("root"), chk.Mode(0755)),                                  // directory itself
		chk.NewPattern(chk.Tree("home/user"), chk.Users("chronos"), chk.Groups("chronos-access"), chk.Mode(0750), chk.SkipChildren()), // top-level children
		chk.NewPattern(chk.Tree("home"), chk.Users("root"), chk.Groups("root"), chk.NotMode(022)),

		chk.NewPattern(chk.Path("unencrypted/apkcache"), chk.Mode(0700), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("unencrypted/attestation"), chk.Users("attestation", "root"), chk.NotMode(022)),
		chk.NewPattern(chk.Path("unencrypted/preserve"), chk.Users("root"), chk.NotMode(02)),                 // directory itself
		chk.NewPattern(chk.Path("unencrypted/preserve/cros-update"), chk.SkipChildren()),                     // only exists for testing
		chk.NewPattern(chk.Path("unencrypted/preserve/log"), chk.SkipChildren()),                             // only exists for testing
		chk.NewPattern(chk.Tree("unencrypted/preserve"), chk.Users("attestation", "root"), chk.NotMode(022)), // other children
		chk.NewPattern(chk.Path("unencrypted/userspace_swap.tmp"), chk.Users("chronos"), chk.SkipChildren()),
		chk.NewPattern(chk.Tree("unencrypted"), chk.Users("root"), chk.NotMode(022)),

		chk.NewPattern(chk.Path("var_overlay"), chk.SkipChildren()), // only exists for dev images

		// This file can be created by
		// https://source.corp.google.com/chromeos_public/src/platform/factory/py/gooftool/wipe.py.
		// TODO(crbug.com/1083285): Avoid creating this file with 0666 permissions.
		chk.NewPattern(chk.Path("wipe_mark_file"), chk.Mode(0666)),

		chk.NewPattern(chk.Root(), chk.Users("root"), chk.Groups("root"), chk.Mode(0755)), // stateful_partition directory itself
		chk.NewPattern(chk.AllPaths(), chk.Users("root"), chk.NotMode(022)),               // everything else not already matched
	}

	// prependPatterns prepends the supplied patterns to the main patterns slice.
	prependPatterns := func(newPatterns ...*chk.Pattern) { patterns = append(newPatterns, patterns...) }

	if _, err := user.Lookup("tss"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("var-overlay/lib/tpm"), chk.Users("tss"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("tpm_manager"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Path("encrypted/var/lib/tpm_manager"), chk.Users("tpm_manager"), chk.Groups("tpm_manager"), chk.NotMode(022)),
			chk.NewPattern(chk.Path("encrypted/var/lib/tpm_manager/local_tpm_data"), chk.Users("root"), chk.Groups("root"), chk.NotMode(077)))
	}

	if _, err := user.Lookup("tpm2-simulator"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Path("unencrypted/tpm2-simulator"), chk.Users("tpm2-simulator"), chk.Groups("tpm2-simulator"), chk.NotMode(022)),
			chk.NewPattern(chk.Path("unencrypted/tpm2-simulator/NVChip"), chk.Users("root"), chk.Groups("root"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("trunks"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Path("encrypted/var/lib/trunks"), chk.Users("trunks"), chk.Groups("trunks"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("biod"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Tree("encrypted/var/log/bio_crypto_init"), chk.Users("biod", "root"), chk.Groups("biod", "root"), chk.NotMode(022)),
			chk.NewPattern(chk.Tree("encrypted/var/log/biod"), chk.Users("biod", "root"), chk.Groups("biod", "root"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("buffet"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/lib/buffet"), chk.Users("buffet"), chk.Groups("buffet"), chk.NotMode(02)))
	}

	if _, err := user.Lookup("cups"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Tree("encrypted/var/cache/cups"), chk.Users("cups"), chk.Groups("cups", "nobody"), chk.NotMode(02)),
			chk.NewPattern(chk.Tree("encrypted/var/spool/cups"), chk.Users("cups"), chk.Groups("cups", "nobody"), chk.NotMode(02)))
	}

	if _, err := user.Lookup("android-root"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Tree("unencrypted/art-data"), chk.Users("android-root", "root"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("cdm-oemcrypto"); err == nil {
		prependPatterns(chk.NewPattern(chk.Path("encrypted/var/lib/oemcrypto"), chk.Users("cdm-oemcrypto"), chk.Groups("cdm-oemcrypto"), chk.Mode(0700), chk.SkipChildren()))
	}

	if _, err := user.Lookup("fwupd"); err == nil {
		prependPatterns(
			chk.NewPattern(chk.Path("encrypted/var/cache/fwupd"), chk.Users("fwupd"), chk.Groups("fwupd"), chk.SkipChildren()),
			chk.NewPattern(chk.Path("encrypted/var/lib/fwupd"), chk.Users("fwupd"), chk.Groups("fwupd"), chk.SkipChildren()))
	}

	if _, err := user.Lookup("dlcservice"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/cache/dlc"), chk.Users("dlcservice"), chk.Groups("dlcservice"), chk.NotMode(022)))
		// encrypted dlc-images is created by dev_utils.sh script prior to bind
		// mounting unencrypted dlc-images directory.
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/cache/dlc-images"), chk.Users("root"), chk.Groups("root"), chk.NotMode(022)))
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/lib/dlcservice"), chk.Users("dlcservice"), chk.Groups("dlcservice"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("wilco_dtc"); err == nil {
		prependPatterns(chk.NewPattern(chk.Path("encrypted/var/lib/wilco/storage.img"), chk.Users("wilco_dtc"), chk.Groups("wilco_dtc"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("cros_healthd"); err == nil {
		prependPatterns(chk.NewPattern(chk.Path("encrypted/var/cache/diagnostics"), chk.Users("cros_healthd"), chk.Groups("cros_healthd"), chk.SkipChildren()))
	}

	if _, err := user.Lookup("missived"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/cache/reporting"), chk.Users("missived"), chk.Groups("missived"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("displaylink"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/log/displaylink"), chk.Users("displaylink"), chk.Groups("displaylink"), chk.NotMode(022), chk.SkipChildren()))
	}

	if _, err := user.Lookup("sound_card_init"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/lib/sound_card_init"), chk.Users("sound_card_init"), chk.Groups("sound_card_init"), chk.NotMode(022)))
	}

	if _, err := user.Lookup("rmtfs"); err == nil {
		prependPatterns(chk.NewPattern(chk.Tree("encrypted/var/lib/rmtfs"), chk.Users("rmtfs"), chk.NotMode(022)))
	}

	if moblab.IsMoblab() {
		// On moblab devices, there are additional user dirs and tons of stuff (MySQL, etc.) in /var.
		prependPatterns(
			chk.NewPattern(chk.Tree("home/chronos"), chk.Users("chronos", "root")),
			chk.NewPattern(chk.Tree("home/moblab"), chk.Users("moblab", "root")),
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
