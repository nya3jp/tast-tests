// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package strace

import (
	"testing"
)

func TestMinijailPolicyGenerator_AddSyscall(t *testing.T) {
	input := []struct {
		s string // Syscall.
		p string // Paramters.
		a string // argument to log.
	}{
		{s: "execve", p: `/bin/echo", ["echo", "test"], 0x7fff3c1dc228 /* 49 vars */`, a: ""},
		{s: "brk", p: "NULL", a: ""},
		{s: "openat", p: `AT_FDCWD, "/etc/ld.so.cache", O_RDONLY|O_CLOEXEC`, a: ""},
		{s: "fstat", p: "3, {st_mode=S_IFREG|0644, st_size=62736, ...}", a: ""},
		{s: "mmap", p: "NULL, 62736, PROT_READ, MAP_PRIVATE, 3, 0", a: "PROT_READ"},
		{s: "prctl", p: "PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0", a: "PR_SET_NO_NEW_PRIVS"},
		{s: "mprotect", p: "0x7f86fb98d000, 2097152, PROT_NONE", a: "PROT_NONE"},
		{s: "ioctl", p: "0, TCGETS, 0xffeb2a18", a: "TCGETS"},
	}

	gen := NewMinijailPolicyGenerator()
	for _, args := range input {
		gen.AddSyscall(args.s, args.p)
		entry, ok := gen.frequencyData[args.s]
		if !ok {
			t.Errorf("AddSyscall(%s) failed", args.s)
			continue
		}
		f := 0
		switch entry.(type) {
		case int:
			if len(args.a) != 0 {
				t.Errorf("Expected argument filtering for %q", args.s)
				continue
			}
			f = entry.(int)
		case *argData:
			if len(args.a) == 0 {
				t.Errorf("Did not expect argument filtering for %q", args.s)
				continue
			}
			d := entry.(*argData)
			if _, ok := d.argValues[args.a]; !ok {
				t.Errorf("For %q the expected argument %q was not recorded in %v",
					args.s, args.a, d.argValues)
				continue
			}
			f = d.occurences
		default:
			t.Errorf("Got unexpected type for %q", args.s)
			continue
		}
		if f <= 0 {
			t.Errorf("%q should have a positive frequency", args.s)
			continue
		}
	}
}

func TestMinijailPolicyGenerator_LookupSyscall(t *testing.T) {
	input := []struct {
		s string // Syscall.
		p string // Paramters.
		l string // Lookup result.
	}{
		{s: "execve", p: `/bin/echo", ["echo", "test"], 0x7fff3c1dc228 /* 49 vars */`, l: "execve: 1"},
		{s: "brk", p: "NULL", l: "brk: 1"},
		{s: "access", p: `"/etc/ld.so.preload", R_OK`, l: "access: 1"},
		{s: "openat", p: `AT_FDCWD, "/etc/ld.so.cache", O_RDONLY|O_CLOEXEC`, l: "openat: 1"},
		{s: "fstat", p: "3, {st_mode=S_IFREG|0644, st_size=62736, ...}", l: "fstat: 1"},
		{s: "mmap", p: "NULL, 62736, PROT_READ, MAP_PRIVATE, 3, 0", l: "mmap: arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE"},
		{s: "close", p: "3", l: "close: 1"},
		{s: "read", p: `3, "\177ELF\2\1\1\3\0\0\0\0\0\0\0\0\3\0>\0\1\0\0\0\260\33\2\0\0\0\0\0"..., 832`, l: "read: 1"},
		{s: "mprotect", p: "0x7f86fb98d000, 2097152, PROT_NONE", l: "mprotect: arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE"},
		{s: "arch_prctl", p: "ARCH_SET_FS, 0x7f86fbdad540", l: "arch_prctl: 1"},
		{s: "munmap", p: "0x7f86fbdae000, 62736", l: "munmap: 1"},
		{s: "write", p: `1, "test\n", 5`, l: "write: 1"},
		{s: "ioctl", p: "0, TCGETS, 0xffeb2a18", l: "ioctl: arg1 == TCGETS"},
		{s: "exit_group", p: "0", l: "exit_group: 1"},
	}

	gen := NewMinijailPolicyGenerator()
	for i, args := range input {
		gen.AddSyscall(args.s, args.p)
		if i%2 == 0 {
			gen.AddSyscall(args.s, args.p)
		}
	}

	for i, args := range input {
		expected := 1
		if i%2 == 0 {
			expected = 2
		}
		f, l := gen.LookupSyscall(args.s)
		if f != expected {
			t.Errorf("LookupSyscall(%s) failed got %d expected %d.", args.s, f, expected)
		}
		if l != args.l {
			t.Errorf("LookupSyscall(%s) got unexpected result %q instead of %q.", args.s, l, args.l)
		}
	}
}

func TestMinijailPolicyGenerator_GeneratePolicy(t *testing.T) {
	gen := NewMinijailPolicyGenerator()
	gen.frequencyData = map[string]interface{}{
		"read":  int(1),
		"write": int(1),
		"exit":  int(1),
		"poll":  int(5),
		"ioctl": &argData{occurences: 3, argIndex: 2, argValues: map[string]struct{}{
			"TCGETS":       struct{}{},
			"SIOCGIFFLAGS": struct{}{},
			"SIOCSIFFLAGS": struct{}{},
		}},
	}

	expected := `poll: 1
ioctl: arg2 == SIOCGIFFLAGS || arg2 == SIOCSIFFLAGS || arg2 == TCGETS
exit: 1
read: 1
write: 1
`
	actual := gen.GeneratePolicy()
	if actual != expected {
		t.Errorf("GeneratePolicy resulted in:\n%s\ninstead of\n%s", actual, expected)
	}
}
