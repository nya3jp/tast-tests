// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package strace

import (
	"context"
	"io"
	"testing"
)

func Test_parseLine(t *testing.T) {
	cases := []struct {
		l string // line
		s string // expected syscall
		p string // expected parameters
	}{
		{l: `5027  read(3, "\177ELF\1\1\1\0\0\0\0\0\0\0\0\0\3\0(\0\1\0\0\0\0\0\0\0004\0\0\0"..., 512) = 512`, s: "read",
			p: `3, "\177ELF\1\1\1\0\0\0\0\0\0\0\0\0\3\0(\0\1\0\0\0\0\0\0\0004\0\0\0"..., 512`},
		{l: "5028  ioctl(0, TCGETS, {B38400 opost isig icanon echo ...}) = 0", s: "ioctl",
			p: "0, TCGETS, {B38400 opost isig icanon echo ...}"},
		{l: `openat(AT_FDCWD, "/etc/usbguard/rules.d", O_RDONLY|O_NONBLOCK|O_LARGEFILE|O_CLOEXEC|O_DIRECTORY) = 5`,
			s: "openat", p: `AT_FDCWD, "/etc/usbguard/rules.d", O_RDONLY|O_NONBLOCK|O_LARGEFILE|O_CLOEXEC|O_DIRECTORY`},
		{l: "mmap2(NULL, 483312, PROT_READ|PROT_EXEC, MAP_PRIVATE|MAP_DENYWRITE, 3, 0) = 0xe91a9000", s: "mmap2",
			p: "NULL, 483312, PROT_READ|PROT_EXEC, MAP_PRIVATE|MAP_DENYWRITE, 3, 0"},
	}

	for _, c := range cases {
		s, p := parseLine(c.l)
		if s != c.s {
			t.Errorf("parseLine yielded syscall '%s' instead of '%s'.", s, c.s)
		}
		if p != c.p {
			t.Errorf("parseLine yielded parameters '%s' instead of '%s'.", p, c.p)
		}
	}
}

func applyResultToPolicyGeneratorNoexecCase(ctx context.Context, t *testing.T, filter Filter) {
	cmd := CommandContext(ctx, "/sbin/minijail0", "-n", "/bin/echo", "test string")

	cmd.straceLog.WriteString(`5027  read(3, "\177ELF\1\1\1\0\0\0\0\0\0\0\0\0\3\0(\0\1\0\0\0\0\0\0\0004\0\0\0"..., 512) = 512
5027  read(3, "\177ELF\1\1\1\0\0\0\0\0\0\0\0\0\3\0(\0\1\0\0\0\0\0\0\0004\0\0\0"..., 512) = 512
5028  prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0) = 0
5027  read(3, "\177ELF\1\1\1\0\0\0\0\0\0\0\0\0\3\0(\0\1\0\0\0\0\0\0\0004\0\0\0"..., 512) = 512
mmap2(NULL, 483312, PROT_READ|PROT_EXEC, MAP_PRIVATE|MAP_DENYWRITE, 3, 0) = 0xe91a9000
5028  ioctl(0, TCGETS, {B38400 opost isig icanon echo ...}) = 0
5028  ioctl(3, SIOCGIFFLAGS, {ifr_name="lo", ifr_flags=IFF_LOOPBACK}) = 0
5028  ioctl(3, SIOCSIFFLAGS, {ifr_name="lo", ifr_flags=IFF_UP|IFF_LOOPBACK|IFF_RUNNING}) = 0
`)
	cmd.straceLog.Seek(0, io.SeekStart)

	gen := NewMinijailPolicyGenerator()
	cmd.ApplyResultToPolicyGenerator(gen, filter)

	type expect struct {
		s string // syscall
		f int    // frequency
		p string // policy
	}
	expectations := []expect{
		{s: "mmap2", f: 1, p: "mmap2: arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE"},
		{s: "ioctl", f: 3, p: "ioctl: arg1 == SIOCGIFFLAGS || arg1 == SIOCSIFFLAGS || arg1 == TCGETS"},
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

func applyResultToPolicyGeneratorExecCase(ctx context.Context, t *testing.T, filter Filter) {
	cmd := CommandContext(ctx, "/sbin/minijail0", "-n", "/bin/echo", "test string")
	if err := cmd.Run(); err != nil {
		t.Fatalf("'%v' '%v' failed with %v", cmd.Path, cmd.Args, err)
	}

	gen := NewMinijailPolicyGenerator()
	cmd.ApplyResultToPolicyGenerator(gen, filter)

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

func TestCmd_ApplyResultToPolicyGenerator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

	applyResultToPolicyGeneratorNoexecCase(ctx, t, IncludeAllSyscalls)
	applyResultToPolicyGeneratorNoexecCase(ctx, t, ExcludeSyscallsBeforeSandboxing)

	applyResultToPolicyGeneratorExecCase(ctx, t, IncludeAllSyscalls)
	applyResultToPolicyGeneratorExecCase(ctx, t, ExcludeSyscallsBeforeSandboxing)

}
