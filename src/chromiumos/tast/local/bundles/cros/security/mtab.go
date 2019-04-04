// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/security/filesetup"
	"chromiumos/tast/local/moblab"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Mtab,
		Desc: "Compares mounted filesystems against a baseline",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
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
			s.Log("Waiting for Android mounts")
			if err := arc.WaitAndroidInit(ctx); err != nil {
				s.Error("Failed waiting for Android mounts: ", err) // non-fatal so we can check other mounts
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
		"/":                                 {regexp.MustCompile("^/dev/root$"), "ext2", "ro"},
		"/dev":                              {nil, "devtmpfs", "rw,nosuid,noexec,mode=755"},
		"/dev/pts":                          {nil, "devpts", "rw,nosuid,noexec,gid=5,mode=620"},
		"/dev/shm":                          {nil, "tmpfs", defaultRW},
		"/home":                             {nil, "ext4", defaultRW},
		"/home/chronos":                     {nil, "ext4", defaultRW},
		"/media":                            {nil, "tmpfs", defaultRW},
		"/mnt/stateful_partition":           {nil, "ext4", defaultRW},
		"/mnt/stateful_partition/encrypted": {nil, "ext4", defaultRW},
		"/opt/google/containers/android/rootfs/android-data/data/dalvik-cache/arm":    {nil, "ext4", defaultRW},
		"/opt/google/containers/android/rootfs/android-data/data/dalvik-cache/x86_64": {nil, "ext4", defaultRW},
		"/opt/google/containers/android/rootfs/android-data/data/dalvik-cache/x86":    {nil, "ext4", defaultRW},
		"/opt/google/containers/android/rootfs/root":                                  {loopDev, "squashfs", "ro"},
		"/opt/google/containers/android/rootfs/root/system/lib/arm":                   {loopDev, "squashfs", "ro,nosuid,nodev"},
		"/opt/google/containers/arc-obb-mounter/mountpoints/container-root":           {loopDev, "squashfs", "ro,noexec"},
		"/opt/google/containers/arc-sdcard/mountpoints/container-root":                {loopDev, "squashfs", "ro,noexec"},
		"/proc":                              {nil, "proc", defaultRW},
		"/run":                               {nil, "tmpfs", defaultRW + ",mode=755"},
		"/run/arc/adbd":                      {nil, "tmpfs", defaultRW + ",mode=770"},
		"/run/arc/debugfs/sync":              {nil, "debugfs", defaultRW},
		"/run/arc/debugfs/tracing":           {nil, "debugfs,tracefs", defaultRW},
		"/run/arc/media":                     {nil, "tmpfs", defaultRO + ",mode=755"},
		"/run/arc/obb":                       {nil, "tmpfs", defaultRO + ",mode=755"},
		"/run/arc/oem":                       {nil, "tmpfs", defaultRW + ",mode=755"},
		"/run/arc/sdcard":                    {nil, "tmpfs", defaultRO + ",mode=755"},
		"/run/arc/shared_mounts":             {nil, "tmpfs", defaultRW + ",mode=755"},
		"/run/debugfs_gpu":                   {nil, "debugfs", defaultRW + ",gid=605,mode=750"}, // debugfs-access
		"/run/imageloader/PepperFlashPlayer": {nil, "squashfs", "ro,nodev,nosuid"},
		"/run/imageloader":                   {nil, "tmpfs", defaultRW},
		"/run/lock":                          {nil, "tmpfs", defaultRW},
		"/run/netns":                         {nil, "tmpfs", defaultRW}, // TODO: avoid creating mountpoint under /run: crbug.com/757953
		"/sys":                               {nil, "sysfs", defaultRW},
		"/sys/fs/cgroup/cpuacct":             {nil, "cgroup", defaultRW},
		"/sys/fs/cgroup/cpu":                 {nil, "cgroup", defaultRW},
		"/sys/fs/cgroup/cpuset":              {nil, "cgroup", defaultRW},
		"/sys/fs/cgroup/devices":             {nil, "cgroup", defaultRW},
		"/sys/fs/cgroup/freezer":             {nil, "cgroup", defaultRW},
		"/sys/fs/cgroup/schedtune":           {nil, "cgroup", defaultRW},
		"/sys/fs/cgroup":                     {nil, "tmpfs", defaultRW + ",mode=755"},
		"/sys/fs/fuse/connections":           {nil, "fusectl", defaultRW},
		"/sys/fs/pstore":                     {nil, "pstore", defaultRW},
		"/sys/fs/selinux":                    {nil, "selinuxfs", "rw,nosuid,noexec"},
		"/sys/kernel/config":                 {nil, "configfs", defaultRW},
		"/sys/kernel/debug":                  {nil, "debugfs", defaultRW},
		"/sys/kernel/debug/tracing":          {nil, "tracefs", defaultRW},
		"/sys/kernel/security":               {nil, "securityfs", defaultRW},
		"/tmp":                               {nil, "tmpfs", defaultRW},
		"/usr/share/oem":                     {nil, "ext4", defaultRO},
		"/var":                               {nil, "ext4", defaultRW},
		"/var/lock":                          {nil, "tmpfs", defaultRW},               // duplicate of /run/lock
		"/var/run":                           {nil, "tmpfs", defaultRW + ",mode=755"}, // duplicate of /run
	}

	// Moblab devices mount external USB storage devices at /mnt/moblab.
	// See the manual linked from https://www.chromium.org/chromium-os/testing/moblab for more details.
	if moblab.IsMoblab() {
		expMounts["/mnt/moblab"] = mountSpec{nil, "ext4", "rw"}
	}

	// Regular expression matching mounts under /run/daemon-store, and corresponding spec.
	daemonStoreRegexp := regexp.MustCompile("^/run/daemon-store/([^/]+)$")
	daemonStoreSpec := mountSpec{nil, "tmpfs", defaultRW + ",mode=755"}

	// Mounts that are modified for dev/test images and thus ignored when checking /etc/mtab.
	ignoredLiveMounts := []string{
		"/home",
		"/tmp",
		"/usr/local",
		"/var/db/pkg",
		"/var/lib/portage",
	}

	// Filesystem types that are skipped.
	ignoredTypes := []string{
		"ecryptfs",
		"nsfs", // kernel filesystem used with network namespaces
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

	const (
		liveMtab  = "/etc/mtab"                  // mtab listing live mounts
		buildMtab = "/var/log/mount_options.log" // mtab captured before modifying for dev/test
	)

	// checkMounts reports non-fatal errors if the mount described in info doesn't match the expected spec.
	checkMount := func(info mountInfo, mtab string) {
		// Skip rootfs since /dev/root is mapped to the same location.
		if info.dev == "rootfs" {
			return
		}
		if inSlice(info.fs, ignoredTypes) {
			return
		}
		// When looking at /etc/mtab, skip mounts that are modified for dev/test images.
		if mtab == liveMtab && inSlice(info.mount, ignoredLiveMounts) {
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
				s.Errorf("Unexpected mount %v in %v with device %q and type %q", info.mount, mtab, info.dev, info.fs)
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

		var missing []string
		for _, o := range strings.Split(exp.options, ",") {
			if !inSlice(o, info.options) {
				missing = append(missing, o)
			}
		}
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
