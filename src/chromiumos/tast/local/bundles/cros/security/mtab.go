// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/security/filesetup"
	"chromiumos/tast/local/moblab"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Mtab,
		Desc: "Compares mounted filesystems against a baseline",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func Mtab(ctx context.Context, s *testing.State) {
	// Give up if the root partition has been remounted read/write (since other mount
	// options are also likely to be incorrect).
	if ro, err := filesetup.ReadOnlyRootPartition(); err != nil {
		s.Fatal("Failed to check if root partition is mounted read-only: ", err)
	} else if !ro {
		s.Fatal("Root partition is mounted read/write; rootfs verification disabled?")
	}

	if upstart.JobExists(ctx, "ui") {
		// Make sure that there's no ongoing user session, as we don't want to see users'
		// encrypted home dirs or miscellaneous ARC mounts.
		s.Log("Restarting ui job to clean up transient mounts")
		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to restart ui job: ", err)
		}
		// Android mounts don't appear immediately after the ui job starts, so wait
		// a bit if the system supports Android.
		if arc.Supported() {
			// TODO(crbug.com/1033637): support ARCVM.
			if t, ok := arc.Type(); ok && t == arc.Container {
				reader, err := syslog.NewReader(ctx)
				if err != nil {
					s.Fatal("Failed to open syslog reader: ", err)
				}
				defer reader.Close()

				s.Log("Waiting for Android mounts")
				if err := arc.WaitAndroidInit(ctx, reader); err != nil {
					s.Error("Failed waiting for Android mounts: ", err) // non-fatal so we can check other mounts
				}
			}
		}
	}

	// mountSpec holds required criteria for a mounted filesystem.
	type mountSpec struct {
		dev     *regexp.Regexp // matches mounted device, or nil to not check
		fs      string         // comma-separated list of filesystem types
		options string         // comma-separated list of required options
	}

	const (
		defaultRW = "rw,nosuid,nodev,noexec" // default options for read/write mounts
		defaultRO = "ro,nosuid,nodev,noexec" // default options for read-only mounts
	)
	loopDev := regexp.MustCompile("^/dev/loop[0-9]+$") // matches loopback devices

	// Specifications used to check mounts. These are ignored if not present.
	expMounts := map[string]mountSpec{
		"/": {regexp.MustCompile("^/dev/root$"), "ext2", "ro"},

		"/dev":     {nil, "devtmpfs", "rw,nosuid,noexec,mode=755"},
		"/dev/pts": {nil, "devpts", "rw,nosuid,noexec,gid=5,mode=620"},

		"/opt/google/containers/android/rootfs/root":                        {loopDev, "squashfs", "ro"},
		"/opt/google/containers/android/rootfs/root/system/lib/arm":         {loopDev, "squashfs", "ro,nosuid,nodev"},
		"/opt/google/containers/arc-obb-mounter/mountpoints/container-root": {loopDev, "squashfs", "ro,noexec"},
		"/opt/google/containers/arc-sdcard/mountpoints/container-root":      {loopDev, "squashfs", "ro,noexec"},

		"/run":                     {nil, "tmpfs", defaultRW + ",mode=755"},
		"/run/arc/adb":             {nil, "tmpfs", defaultRW + ",mode=775"},
		"/run/arc/adbd":            {nil, "tmpfs", defaultRW + ",mode=770"},
		"/run/arc/media":           {nil, "tmpfs", defaultRO + ",mode=755"},
		"/run/arc/obb":             {nil, "tmpfs", defaultRO + ",mode=755"},
		"/run/arc/oem":             {nil, "tmpfs", defaultRW + ",mode=755"},
		"/run/arc/sdcard":          {nil, "tmpfs", defaultRO + ",mode=755"},
		"/run/arc/shared_mounts":   {nil, "tmpfs", defaultRW + ",mode=755"},
		"/run/arc/debugfs/sync":    {nil, "debugfs", defaultRW + ",gid=605,mode=750"},
		"/run/arc/debugfs/tracing": {nil, "debugfs,tracefs", defaultRW},
		"/run/chromeos-config/v1":  {nil, "tmpfs", defaultRO},
		"/run/debugfs_gpu":         {nil, "debugfs", defaultRW + ",gid=605,mode=750"}, // debugfs-access
		"/run/imageloader":         {nil, "tmpfs", defaultRW + ",mode=755"},
		"/run/namespaces":          {nil, "tmpfs", defaultRW + ",mode=755"}, // This is a bind mount
		"/run/netns":               {nil, "tmpfs", defaultRW + ",mode=755"},
		"/run/lock":                {nil, "tmpfs", defaultRW + ",mode=755"},

		"/sys/fs/cgroup":    {nil, "tmpfs", defaultRW + ",mode=755"},
		"/sys/fs/selinux":   {nil, "selinuxfs", "rw,nosuid,noexec"},
		"/sys/kernel/debug": {nil, "debugfs", defaultRW + ",gid=605,mode=750"},

		"/usr/share/chromeos-assets/quickoffice/_platform_specific": {loopDev, "squashfs", defaultRO},
		"/usr/share/chromeos-assets/speech_synthesis/patts":         {loopDev, "squashfs", "nodev,nosuid"},
		"/usr/share/cros-camera/libfs":                              {loopDev, "squashfs", "ro,nosuid,nodev"},

		"/var/lock": {nil, "tmpfs", defaultRW + ",mode=755"}, // duplicate of /run/lock
		"/var/run":  {nil, "tmpfs", defaultRW + ",mode=755"}, // duplicate of /run
	}

	// Moblab devices mount external USB storage devices at several locations.
	// See the manual linked from https://www.chromium.org/chromium-os/testing/moblab for more details.
	if moblab.IsMoblab() {
		expMounts["/mnt/moblab"] = mountSpec{nil, "ext4", "rw"}
		expMounts["/mnt/moblab-settings"] = mountSpec{nil, "ext4", "rw,nosuid"}
		expMounts["/mnt/moblab/containers/docker"] = mountSpec{nil, "ext4", "rw"}
	}

	// Regular expression matching mounts under /run/daemon-store, and corresponding spec.
	daemonStoreRegexp := regexp.MustCompile("^/run/daemon-store/([^/]+)$")
	daemonStoreSpec := mountSpec{nil, "tmpfs", defaultRW + ",mode=755"}

	// Mounts that are modified for dev/test images and thus ignored when checking /etc/mtab.
	ignoredLiveMountPatterns := []string{
		"/home",
		"/tmp",
		"/usr/local",
		"/var/cache/dlc-images",
		"/var/db/pkg",
		"/var/lib/portage",
		// imageloader creates mount point at /run/imageloader/{id}/{package}.
		"/run/imageloader/[^/]+/[^/]+",
	}
	if moblab.IsMoblab() {
		ignoredLiveMountPatterns = append(ignoredLiveMountPatterns, "^/mnt/moblab/containers/docker/.*")
	}

	ignoredLiveMountsRegexp := regexp.MustCompile(fmt.Sprintf("^(%s)$", strings.Join(ignoredLiveMountPatterns, "|")))

	// Filesystem types that are skipped.
	ignoredTypes := []string{
		"ecryptfs",
		"nsfs", // kernel filesystem used with namespaces
		"proc", // TODO(crbug.com/1204115): Re-enable "proc" testing once 3.18 kernels are out.
	}

	// Returns true if s appears in vals.
	inSlice := func(s string, vals []string) bool {
		for _, v := range vals {
			if s == v {
				return true
			}
		}
		return false
	}

	// Returns true if s is a prefix of any value in vals.
	hasPrefixInSlice := func(s string, vals []string) bool {
		for _, v := range vals {
			if strings.HasPrefix(v, s) {
				return true
			}
		}
		return false
	}

	// Returns true if all values in needle are included in haystack, and also
	// returns the set difference (needle - haystack). The function returns true
	// iff the difference is empty.
	included := func(needle, haystack []string) (bool, []string) {
		var difference []string
		for _, o := range needle {
			if !inSlice(o, haystack) {
				difference = append(difference, o)
			}
		}
		return len(difference) == 0, difference
	}

	const (
		liveMtab  = "/etc/mtab"                  // mtab listing live mounts
		buildMtab = "/var/log/mount_options.log" // mtab captured before modifying for dev/test
	)

	// checkMount reports non-fatal errors if the mount described in info doesn't match the expected spec.
	checkMount := func(info mountInfo, mtab string) {
		// Skip rootfs since /dev/root is mapped to the same location.
		if info.dev == "rootfs" {
			return
		}
		if inSlice(info.fs, ignoredTypes) {
			return
		}
		// When looking at /etc/mtab, skip mounts that are modified for dev/test images.
		if mtab == liveMtab && ignoredLiveMountsRegexp.MatchString(info.mount) {
			return
		}

		// Mounts that include either the defaultRO or defaultRW flags *and*
		// no extra mode= or gid= flags are OK.
		okRO, _ := included(strings.Split(defaultRO, ","), info.options)
		okRW, _ := included(strings.Split(defaultRW, ","), info.options)
		if (okRO || okRW) &&
			!hasPrefixInSlice("mode", info.options) &&
			!hasPrefixInSlice("gid", info.options) {
			return
		}

		var exp mountSpec
		if matches := daemonStoreRegexp.FindStringSubmatch(info.mount); matches != nil {
			// Directories in /run/daemon-store should be owned by root.
			if fi, err := os.Stat(info.mount); err != nil {
				s.Errorf("Failed to stat mount %v from %v: %v", info.mount, mtab, err)
			} else if st := fi.Sys().(*syscall.Stat_t); st.Uid != 0 || st.Gid != 0 {
				s.Errorf("Mount %v in %v is owned by %d:%d; want 0:0", info.mount, mtab, st.Uid, st.Gid)
			}
			// They should also have corresponding dirs in /etc/daemon-store.
			etcDir := filepath.Join("/etc/daemon-store", matches[1])
			if _, err := os.Stat(etcDir); err != nil {
				s.Errorf("Mount %v in %v has bad config dir %v: %v", info.mount, mtab, etcDir, err)
			}
			exp = daemonStoreSpec
		} else {
			// All other mounts must be listed in the map.
			var ok bool
			if exp, ok = expMounts[info.mount]; !ok {
				s.Errorf("Unexpected mount %v in %v with device %q, type %q, options %v", info.mount, mtab, info.dev, info.fs, info.options)
				return
			}
		}

		if exp.dev != nil && !exp.dev.MatchString(info.dev) {
			s.Errorf("Mount %v in %v has device %q not matched by %q", info.mount, mtab, info.dev, exp.dev)
		}

		validFSes := strings.Split(exp.fs, ",")
		foundFS := false
		for _, fs := range validFSes {
			if info.fs == fs {
				foundFS = true
				break
			}
		}
		if !foundFS {
			s.Errorf("Mount %v in %v has type %q; want %s", info.mount, mtab, info.fs, validFSes)
		}

		_, missing := included(strings.Split(exp.options, ","), info.options)
		if len(missing) > 0 {
			s.Errorf("Mount %v in %v is missing option(s) %v (has %v)", info.mount, mtab, missing, info.options)
		}
	}

	for _, mtab := range []string{liveMtab, buildMtab} {
		if mounts, err := readMtab(mtab); err != nil {
			s.Errorf("Failed to read %v: %v", mtab, err)
		} else {
			for _, info := range mounts {
				checkMount(info, mtab)
			}
		}
	}
}

// mountInfo describes a row from /etc/mtab.
type mountInfo struct {
	dev, mount, fs string
	options        []string
}

// readMtab reads and parses the mtab file at path (e.g. /etc/mtab).
// It would be nice to use gopsutil's disk.Partitions here, but we need
// to be able to parse /var/log/mount_options.log as well.
func readMtab(path string) ([]mountInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var mounts []mountInfo
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i != -1 {
			line = line[0:i]
		}
		fields := strings.Fields(line)
		if len(fields) != 6 {
			return nil, errors.Errorf("malformed line %q", sc.Text())
		}
		mounts = append(mounts, mountInfo{fields[0], fields[1], fields[2], strings.Split(fields[3], ",")})
	}
	return mounts, sc.Err()
}
