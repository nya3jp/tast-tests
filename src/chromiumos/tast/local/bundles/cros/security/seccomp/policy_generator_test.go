// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package seccomp

import (
	"context"
	"os"
	"testing"
)

func TestPolicyGenerator_AddSyscall(t *testing.T) {
	input := []struct {
		s string // Syscall.
		p string // Parameters.
		a string // Argument to log.
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

	gen := NewPolicyGenerator()
	for _, args := range input {
		gen.AddSyscall(args.s, args.p)
		entry, ok := gen.frequencyData[args.s]
		if !ok {
			t.Errorf("AddSyscall(%s) failed", args.s)
			continue
		}
		if entry.occurences <= 0 {
			t.Errorf("%q should have a positive frequency", args.s)
			continue
		}
		if entry.argIndex < 0 {
			if len(args.a) != 0 {
				t.Errorf("Expected argument filtering for %q", args.s)
				continue
			}
		} else {
			if len(args.a) == 0 {
				t.Errorf("Did not expect argument filtering for %q", args.s)
				continue
			}
			if _, ok := entry.argValues[args.a]; !ok {
				t.Errorf("For %q the expected argument %q was not recorded in %v",
					args.s, args.a, entry.argValues)
				continue
			}
		}
	}
}

func addStraceLogNoexecCase(ctx context.Context, t *testing.T, filter Filter) {
	_, logFile := CommandContext(ctx, "/sbin/minijail0", "-n", "/bin/echo", "test string")
	if len(logFile) == 0 {
		t.Fatal("Failed to get temp file.")
	}

	file, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Create(%q) failed.", logFile)
	}
	_, err = file.WriteString(`5027  read(3, "\177ELF\1\1\1\0\0\0\0\0\0\0\0\0\3\0(\0\1\0\0\0\0\0\0\0004\0\0\0"..., 512) = 512
5027  read(3, "\177ELF\1\1\1\0\0\0\0\0\0\0\0\0\3\0(\0\1\0\0\0\0\0\0\0004\0\0\0"..., 512) = 512
5028  prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0) = 0
5027  read(3, "\177ELF\1\1\1\0\0\0\0\0\0\0\0\0\3\0(\0\1\0\0\0\0\0\0\0004\0\0\0"..., 512) = 512
mmap2(NULL, 483312, PROT_READ|PROT_EXEC, MAP_PRIVATE|MAP_DENYWRITE, 3, 0) = 0xe91a9000
5028  ioctl(0, TCGETS, {B38400 opost isig icanon echo ...}) = 0
5028  ioctl(3, SIOCGIFFLAGS, {ifr_name="lo", ifr_flags=IFF_LOOPBACK}) = 0
5028  ioctl(3, SIOCSIFFLAGS, {ifr_name="lo", ifr_flags=IFF_UP|IFF_LOOPBACK|IFF_RUNNING}) = 0
5028  mprotect(0x7f86fb98d000, 2097152, PROT_READ|PROT_WRITE|PROT_EXEC) = 0x7f86fb98d000
`)
	if err != nil {
		t.Fatal("Failed to write test log file: ", err)
	}
	file.Close()

	gen := NewPolicyGenerator()
	err = gen.AddStraceLog(logFile, filter)
	if err != nil {
		t.Fatal("ApplyResultToPolicyGenerator() failed: ", err)
	}

	type expect struct {
		s string // Syscall.
		f int    // Frequency.
		p string // Policy.
	}
	expectations := []expect{
		{s: "mmap2", f: 1, p: "mmap2: arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE"},
		{s: "ioctl", f: 3, p: "ioctl: arg1 == SIOCGIFFLAGS || arg1 == SIOCSIFFLAGS || arg1 == TCGETS"},
		{s: "mprotect", f: 1, p: "mprotect: arg2 == PROT_READ|PROT_WRITE|PROT_EXEC"},
	}

	switch filter {
	case IncludeAllSyscalls:
		expectations = append(expectations,
			expect{s: "read", f: 3, p: "read: 1"},
			expect{s: "prctl", f: 1, p: "prctl: arg0 == PR_SET_NO_NEW_PRIVS"})
	case ExcludeSyscallsBeforeSandboxing:
		expectations = append(expectations,
			expect{s: "read", f: 1, p: "read: 1"},
			expect{s: "prctl", f: 0, p: ""})
	}

	for _, e := range expectations {
		f, p := gen.LookupSyscall(e.s)
		if f != e.f {
			t.Errorf("LookupSyscall(%s) yeilded frequency %d instead of %d.", e.s, f, e.f)
			continue
		}
		if p != e.p {
			t.Errorf("LookupSyscall(%s) yeilded policy '%s' instead of '%s'.", e.s, p, e.p)
			continue
		}
	}
}

func addStraceLogExecCase(ctx context.Context, t *testing.T, filter Filter) {
	cmd, logFile := CommandContext(ctx, "/sbin/minijail0", "-n", "/bin/echo", "test string")
	if len(logFile) == 0 {
		t.Fatal("Failed to get temp file.")
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("%q %q failed with %v.", cmd.Path, cmd.Args, err)
	}

	gen := NewPolicyGenerator()
	err := gen.AddStraceLog(logFile, filter)
	if err != nil {
		t.Fatal("ApplyResultToPolicyGenerator() failed", err)
	}

	s := "write"
	expected := "write: 1"
	f, l := gen.LookupSyscall(s)
	if f == 0 {
		t.Errorf("LookupSyscall(%s) failed.", s)
	}
	if l != expected {
		t.Errorf("LookupSyscall(%s) got unexpected result '%s' instead of '%s'.", s, l, expected)
	}

	s = "prctl"
	f, l = gen.LookupSyscall(s)
	switch filter {
	case IncludeAllSyscalls:
		if f == 0 {
			t.Errorf("LookupSyscall(%s) unexpectedly failed.", s)
		}
	case ExcludeSyscallsBeforeSandboxing:
		if f != 0 {
			t.Errorf("LookupSyscall(%s) unexpectedly succeeded with a frequency of %d and value '%s'.", s, f, l)
		}
	}
}

func TestPolicyGenerator_AddStraceLog(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

	addStraceLogNoexecCase(ctx, t, IncludeAllSyscalls)
	addStraceLogNoexecCase(ctx, t, ExcludeSyscallsBeforeSandboxing)

	addStraceLogExecCase(ctx, t, IncludeAllSyscalls)
	addStraceLogExecCase(ctx, t, ExcludeSyscallsBeforeSandboxing)

}

func TestPolicyGenerator_LookupSyscall(t *testing.T) {
	input := []struct {
		s string // Syscall.
		p string // Parameters.
		l string // Lookup result.
	}{
		{s: "execve", p: `/bin/echo", ["echo", "test"], 0x7fff3c1dc228 /* 49 vars */`, l: "execve: 1"},
		{s: "brk", p: "NULL", l: "brk: 1"},
		{s: "access", p: `"/etc/ld.so.preload", R_OK`, l: "access: 1"},
		{s: "openat", p: `AT_FDCWD, "/etc/ld.so.cache", O_RDONLY|O_CLOEXEC`, l: "openat: 1"},
		{s: "fstat", p: "3, {st_mode=S_IFREG|0644, st_size=62736, ...}", l: "fstat: 1"},
		{s: "mmap", p: "NULL, 62736, PROT_READ, MAP_PRIVATE, 3, 0", l: "mmap: arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE"},
		{s: "mmap2", p: "NULL, 62736, PROT_WRITE|PROT_EXEC, MAP_PRIVATE, 3, 0", l: "mmap2: arg2 == PROT_WRITE|PROT_EXEC"},
		{s: "close", p: "3", l: "close: 1"},
		{s: "read", p: `3, "\177ELF\2\1\1\3\0\0\0\0\0\0\0\0\3\0>\0\1\0\0\0\260\33\2\0\0\0\0\0"..., 832`, l: "read: 1"},
		{s: "mprotect", p: "0x7f86fb98d000, 2097152, PROT_NONE", l: "mprotect: arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE"},
		{s: "arch_prctl", p: "ARCH_SET_FS, 0x7f86fbdad540", l: "arch_prctl: 1"},
		{s: "munmap", p: "0x7f86fbdae000, 62736", l: "munmap: 1"},
		{s: "write", p: `1, "test\n", 5`, l: "write: 1"},
		{s: "ioctl", p: "0, TCGETS, 0xffeb2a18", l: "ioctl: arg1 == TCGETS"},
		{s: "exit_group", p: "0", l: "exit_group: 1"},
	}

	gen := NewPolicyGenerator()
	for i, args := range input {
		gen.AddSyscall(args.s, args.p)

		// For even indexed system calls, add them twice to make sure they are counted properly.
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
			t.Errorf("LookupSyscall(%s) failed: got %d; want %d.", args.s, f, expected)
		}
		if l != args.l {
			t.Errorf("LookupSyscall(%s) failed: got %q; want %q.", args.s, l, args.l)
		}
	}
}

func TestPolicyGenerator_GeneratePolicy(t *testing.T) {
	gen := NewPolicyGenerator()
	gen.frequencyData = map[string]*argData{
		"read":  {1, -1, map[string]struct{}{}, false},
		"write": {1, -1, map[string]struct{}{}, false},
		"exit":  {1, -1, map[string]struct{}{}, false},
		"poll":  {5, -1, map[string]struct{}{}, false},
		"ioctl": {occurences: 3, argIndex: 2, argValues: map[string]struct{}{
			"TCGETS":       {},
			"SIOCGIFFLAGS": {},
			"SIOCSIFFLAGS": {},
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
		t.Errorf("GeneratePolicy() failed: got\n%q\nwant\n%q", actual, expected)
	}
}
