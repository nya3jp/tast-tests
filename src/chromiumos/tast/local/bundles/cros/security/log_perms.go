// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"os"
	"os/user"
	"strconv"
	"syscall"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LogPerms,
		Desc: "Checks permissions of logging-related files",
		Attr: []string{"bvt"},
	})
}

func LogPerms(s *testing.State) {
	u, err := user.Lookup("syslog")
	if err != nil {
		s.Fatal("No syslog user:", err)
	}
	g, err := user.LookupGroup("syslog")
	if err != nil {
		s.Fatal("No syslog group:", err)
	}

	if u.Gid != g.Gid {
		s.Errorf("syslog user's primary group (%s) isn't syslog (%s)", u.Gid, g.Gid)
	}

	if fi, err := os.Stat("/var/log"); err != nil {
		s.Error(err)
	} else {
		if fi.Mode()&os.ModeSticky == 0 {
			s.Error("/var/log doesn't have sticky bit set")
		}
		if gid := fi.Sys().(*syscall.Stat_t).Gid; strconv.Itoa(int(gid)) != g.Gid {
			s.Errorf("/var/log not owned by syslog group (got %d; want %s)", gid, g.Gid)
		}
	}

	if fi, err := os.Stat("/var/log/messages"); err != nil {
		// The file is briefly missing during log rotation.
		if !os.IsNotExist(err) {
			s.Error(err)
		}
	} else {
		uid := fi.Sys().(*syscall.Stat_t).Uid
		// The new file is briefly owned by root during log rotation.
		if strconv.Itoa(int(uid)) != u.Uid && !(uid == 0 && fi.Size() == 0) {
			s.Errorf("/var/log/messages not owned by syslog user (got %d; want %s)", uid, u.Uid)
		}
	}
}
