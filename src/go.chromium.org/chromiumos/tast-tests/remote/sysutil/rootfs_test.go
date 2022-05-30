// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sysutil

import (
	"testing"
)

const (
	procMountsRW = `/dev/root / ext2 rw,seclabel,relatime 0 0
devtmpfs /dev devtmpfs rw,seclabel,nosuid,noexec,relatime,size=4018320k,nr_inodes=1004580,mode=755 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
sysfs /sys sysfs rw,seclabel,nosuid,nodev,noexec,relatime 0 0
tmpfs /tmp tmpfs rw,seclabel,nosuid,nodev,noexec,relatime 0 0
tmpfs /run tmpfs rw,seclabel,nosuid,nodev,noexec,relatime,mode=755 0 0
`
	procMountsRO = `/dev/root / ext2 ro,seclabel,relatime 0 0
devtmpfs /dev devtmpfs rw,seclabel,nosuid,noexec,relatime,size=4018320k,nr_inodes=1004580,mode=755 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
sysfs /sys sysfs rw,seclabel,nosuid,nodev,noexec,relatime 0 0
tmpfs /tmp tmpfs rw,seclabel,nosuid,nodev,noexec,relatime 0 0
tmpfs /run tmpfs rw,seclabel,nosuid,nodev,noexec,relatime,mode=755 0 0
`

	procMountsReorderedRO = `devtmpfs /dev devtmpfs rw,seclabel,nosuid,noexec,relatime,size=4018320k,nr_inodes=1004580,mode=755 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
sysfs /sys sysfs rw,seclabel,nosuid,nodev,noexec,relatime 0 0
/dev/root / ext2 ro,seclabel,relatime 0 0
tmpfs /tmp tmpfs rw,seclabel,nosuid,nodev,noexec,relatime 0 0
tmpfs /run tmpfs rw,seclabel,nosuid,nodev,noexec,relatime,mode=755 0 0
`
)

func TestIsRootfsWritable(t *testing.T) {
	for _, e := range []struct {
		procMounts string
		writable   bool
	}{
		{procMountsRW, true},
		{procMountsRO, false},
		{procMountsReorderedRO, false},
	} {
		if writable, err := isRootfsWritable(e.procMounts); err != nil {
			t.Errorf("isRootfsWritable(%q) failed: %v", e.procMounts, err)
		} else if writable != e.writable {
			t.Errorf("isRootfsWritable(%q) failed: got %t; want %t", e.procMounts, writable, e.writable)
		}
	}
}
