// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/security/fscaps"
	"chromiumos/tast/local/moblab"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PrivilegedFiles,
		Desc: "Compares files' setuid/setgid bits and capabilities against a baseline",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
	})
}

func PrivilegedFiles(ctx context.Context, s *testing.State) {
	// Directories that should not be examined.
	skippedDirs := makeStringSet([]string{
		"/proc",
		"/dev",
		"/sys",
		"/run",
		"/usr/local",
		"/mnt/stateful_partition",
	})

	// Files permitted to have the setuid and setgid bits set.
	// No error is reported if these files are missing or don't have the bit.
	setuidBaseline := makeStringSet([]string{
		"/usr/bin/powerd_setuid_helper",
		"/usr/bin/sudo",
		"/usr/libexec/dbus-daemon-launch-helper",

		// Some boards (e.g. betty, novato) are released only as VM images with
		// _userdebug ARC++ images which have more binaries setuid than _user images.
		// _userdebug images are required for adb to work out of the box and for mesa to
		// use llvmpipe graphics software rendering in VMs (b/33072485).
		"/opt/google/containers/android/rootfs/root/system/xbin/librank",
		"/opt/google/containers/android/rootfs/root/system/xbin/procmem",
		"/opt/google/containers/android/rootfs/root/system/xbin/procrank",
		"/opt/google/containers/android/rootfs/root/system/xbin/su",
	})
	setgidBaseline := makeStringSet([]string{
		// See setuidBaseline for context.
		"/opt/google/containers/android/rootfs/root/system/xbin/librank",
		"/opt/google/containers/android/rootfs/root/system/xbin/procmem",
		"/opt/google/containers/android/rootfs/root/system/xbin/procrank",
	})

	// Additional files exist on moblab boards.
	if moblab.IsMoblab() {
		setuidBaseline["/usr/libexec/lxc/lxc-user-nic"] = struct{}{}
		setgidBaseline["/usr/bin/screen-4.6.1"] = struct{}{}
	}

	// Files permitted to have Linux capabilities. No error is reported if these files
	// are missing or have no capabilities, but if they exist and have capabilities, they
	// must exactly match the ones specified here.
	capsBaseline := map[string]fscaps.Caps{
		"/bin/arping": fscaps.Caps{Effective: fscaps.NET_RAW, Permitted: fscaps.NET_RAW},
		"/bin/ping":   fscaps.Caps{Effective: fscaps.NET_RAW, Permitted: fscaps.NET_RAW},
		"/bin/ping6":  fscaps.Caps{Effective: fscaps.NET_RAW, Permitted: fscaps.NET_RAW},
		"/opt/google/containers/android/rootfs/root/system/bin/logd": fscaps.Caps{
			Effective: fscaps.SETGID | fscaps.AUDIT_CONTROL,
			Permitted: fscaps.SETGID | fscaps.AUDIT_CONTROL,
		},
		"/opt/google/containers/android/rootfs/root/system/bin/run-as": fscaps.Caps{
			Effective: fscaps.SETGID | fscaps.SETUID,
			Permitted: fscaps.SETGID | fscaps.SETUID,
		},
		"/opt/google/containers/android/rootfs/root/system/bin/simpleperf_app_runner": fscaps.Caps{
			Effective: fscaps.SETGID | fscaps.SETUID,
			Permitted: fscaps.SETGID | fscaps.SETUID,
		},
		"/opt/google/containers/android/rootfs/root/system/bin/surfaceflinger": fscaps.Caps{
			Effective: fscaps.SYS_NICE,
			Permitted: fscaps.SYS_NICE,
		},
		"/opt/google/containers/android/rootfs/root/system/bin/webview_zygote32": fscaps.Caps{
			Effective: fscaps.SETGID | fscaps.SETUID | fscaps.SETPCAP,
			Permitted: fscaps.SETGID | fscaps.SETUID | fscaps.SETPCAP,
		},
		"/sbin/unix_chkpwd":   fscaps.Caps{Effective: fscaps.DAC_OVERRIDE, Permitted: fscaps.DAC_OVERRIDE},
		"/usr/bin/fusermount": fscaps.Caps{Effective: fscaps.SYS_ADMIN, Permitted: fscaps.SYS_ADMIN},
		"/usr/sbin/dnsmasq": fscaps.Caps{
			Effective:   fscaps.NET_ADMIN | fscaps.NET_BIND_SERVICE | fscaps.NET_RAW,
			Inheritable: fscaps.NET_ADMIN | fscaps.NET_BIND_SERVICE | fscaps.NET_RAW,
		},
		"/usr/sbin/hostapd": fscaps.Caps{
			Effective:   fscaps.NET_ADMIN | fscaps.NET_RAW,
			Inheritable: fscaps.NET_ADMIN | fscaps.NET_RAW,
		},
	}

	const maxErrors = 10 // (approximately) the maximum number of errors to report

	numFiles := 0
	var fileErrs []string // error messages to report later
	walkErr := filepath.Walk("/", func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			// Ignore errors, which typically indicate that a file or dir was removed mid-walk.
			return nil
		}
		if _, ok := skippedDirs[path]; ok {
			return filepath.SkipDir
		}
		if fi.IsDir() {
			return nil
		}

		if fi.Mode()&os.ModeSetuid != 0 {
			if _, ok := setuidBaseline[path]; !ok {
				fileErrs = append(fileErrs, path+" has unexpected setuid bit")
			}
		}
		if fi.Mode()&os.ModeSetgid != 0 {
			if _, ok := setgidBaseline[path]; !ok {
				fileErrs = append(fileErrs, path+" has unexpected setgid bit")
			}
		}
		if caps, err := fscaps.GetCaps(path); os.IsNotExist(err) {
			// This probably means that the file was removed.
		} else if err != nil {
			fileErrs = append(fileErrs, fmt.Sprintf("Failed to get capabilities for %v: %v", path, err))
		} else if exp := capsBaseline[path]; !caps.Empty() && caps != exp {
			fileErrs = append(fileErrs, fmt.Sprintf("%v has capabilities %v; want %v", path, caps, exp))
		}

		numFiles++
		if len(fileErrs) > maxErrors {
			return errors.New("too many errors")
		}
		return nil
	})

	s.Logf("Examined %d files", numFiles)
	for _, msg := range fileErrs {
		s.Error(msg)
	}
	if walkErr != nil {
		s.Error("Failed walking filesystem: ", walkErr)
	}
}

// makeStringSet converts strs to a set for faster lookup.
func makeStringSet(strs []string) map[string]struct{} {
	m := make(map[string]struct{}, len(strs))
	for _, s := range strs {
		m[s] = struct{}{}
	}
	return m
}
