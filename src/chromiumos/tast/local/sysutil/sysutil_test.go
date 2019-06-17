// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sysutil

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestUname(t *testing.T) {
	origUnameFunc := unameFunc
	unameFunc = func(u *unix.Utsname) error {
		// Linux
		u.Sysname = [65]byte{76, 105, 110, 117, 120}
		// localhost
		u.Nodename = [65]byte{108, 111, 99, 97, 108, 104, 111, 115, 116}
		// 4.14.118-12008-gee216661cc77
		u.Release = [65]byte{52, 46, 49, 52, 46, 49, 49, 56, 45, 49, 50, 48, 48, 56, 45, 103, 101, 101, 50, 49, 54, 54, 54, 49, 99, 99, 55, 55}
		// #1 SMP PREEMPT Sat May 18 02:25:51 PDT 2019
		u.Version = [65]byte{35, 49, 32, 83, 77, 80, 32, 80, 82, 69, 69, 77, 80, 84, 32, 83, 97, 116, 32, 77, 97, 121, 32, 49, 56, 32, 48, 50, 58, 50, 53, 58, 53, 49, 32, 80, 68, 84, 32, 50, 48, 49, 57}
		// x86_64
		u.Machine = [65]byte{120, 56, 54, 95, 54, 52}
		// (none)
		u.Domainname = [65]byte{40, 110, 111, 110, 101, 41}
		return nil
	}
	defer func() { unameFunc = origUnameFunc }()

	wants := []string{
		"Linux",
		"localhost",
		"4.14.118-12008-gee216661cc77",
		"#1 SMP PREEMPT Sat May 18 02:25:51 PDT 2019",
		"x86_64",
		"(none)",
	}
	res, err := Uname()
	if err != nil {
		t.Error("Uname returned error: ", err)
	}
	gots := []string{
		res.Sysname,
		res.Nodename,
		res.Release,
		res.Version,
		res.Machine,
		res.Domainname,
	}

	for i, got := range gots {
		if want := wants[i]; want != got {
			t.Errorf("%d-th entry: want %s, got %s", i, want, got)
		}
	}
}
