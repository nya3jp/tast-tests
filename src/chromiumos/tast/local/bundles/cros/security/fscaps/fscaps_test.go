// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fscaps

import (
	"fmt"
	"testing"
)

func TestCapsStringAndEmpty(t *testing.T) {
	for _, tc := range []struct {
		eff, inh, perm uint64
		exp            string // caps are sorted by their defined order when printed
		empty          bool
	}{
		{0, 0, 0, "[]", true},
		{KILL, 0, 0, "[e:kill]", false},
		{SYS_PTRACE, 0, SETPCAP, "[e:sys_ptrace p:setpcap]", false},
		{SETPCAP, CHOWN, WAKE_ALARM, "[e:setpcap i:chown p:wake_alarm]", false},
		{SETPCAP | AUDIT_READ, CHOWN, WAKE_ALARM | SYS_ADMIN | SYS_CHROOT,
			"[e:setpcap|audit_read i:chown p:sys_chroot|sys_admin|wake_alarm]", false},
	} {
		c := Caps{Effective: tc.eff, Inheritable: tc.inh, Permitted: tc.perm}
		desc := fmt.Sprintf("Caps{0x%x 0x%x 0x%x}", tc.eff, tc.inh, tc.perm)
		if s := c.String(); s != tc.exp {
			t.Errorf("%s.String() = %q; want %q", desc, s, tc.exp)
		}
		if empty := c.Empty(); empty != tc.empty {
			t.Errorf("%s.Empty() = %v; want %v", desc, empty, tc.empty)
		}
	}
}
