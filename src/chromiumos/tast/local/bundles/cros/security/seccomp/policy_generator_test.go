// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package seccomp

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

func TestPolicyGeneratorAddSyscall(t *testing.T) {
	input := []struct {
		syscall string // System call.
		params  string // Parameters.
		toLog   string // Argument to log.
	}{
		{syscall: "execve", params: `/bin/echo", ["echo", "test"], 0x7fff3c1dc228 /* 49 vars */`, toLog: ""},
		{syscall: "brk", params: "NULL", toLog: ""},
		{syscall: "openat", params: `AT_FDCWD, "/etc/ld.so.cache", O_RDONLY|O_CLOEXEC`, toLog: ""},
		{syscall: "fstat", params: "3, {st_mode=S_IFREG|0644, st_size=62736, ...}", toLog: ""},
		{syscall: "mmap", params: "NULL, 62736, PROT_READ, MAP_PRIVATE, 3, 0", toLog: "PROT_READ"},
		{syscall: "prctl", params: "PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0", toLog: "PR_SET_NO_NEW_PRIVS"},
		{syscall: "mprotect", params: "0x7f86fb98d000, 2097152, PROT_NONE", toLog: "PROT_NONE"},
		{syscall: "ioctl", params: "0, TCGETS, 0xffeb2a18", toLog: "TCGETS"},
	}

	gen := NewPolicyGenerator()
	for _, args := range input {
		gen.AddSyscall(args.syscall, args.params)
		entry, ok := gen.frequencyData[args.syscall]
		if !ok {
			t.Errorf("AddSyscall(%s) failed", args.syscall)
			continue
		}
		if entry.occurences <= 0 {
			t.Errorf("%q should have a positive frequency", args.syscall)
			continue
		}
		if entry.argIndex < 0 {
			if len(args.toLog) != 0 {
				t.Errorf("Expected argument filtering for %q", args.syscall)
				continue
			}
		} else {
			if len(args.toLog) == 0 {
				t.Errorf("Did not expect argument filtering for %q", args.syscall)
				continue
			}
			if _, ok := entry.argValues[args.toLog]; !ok {
				t.Errorf("For %q the expected argument %q was not recorded in %v",
					args.syscall, args.toLog, entry.argValues)
				continue
			}
		}
	}
}

func setupTestLog(t *testing.T, content string) (string, error) {
	logFile, err := ioutil.TempFile("", "tast_strace_")
	if err != nil {
		return "", errors.Wrap(err, "failed to get a temp file")
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			t.Error("Failed to close a temp file: ", err)
		}
	}()

	if len(content) != 0 {
		if _, err := logFile.WriteString(content); err != nil {
			if err := os.Remove(logFile.Name()); err != nil {
				t.Errorf("Remove(%q) failed: %v", logFile.Name(), err)
			}
			return "", errors.Wrap(err, "failed to write test log file")
		}
	}
	return logFile.Name(), nil
}

func testAddStraceLogNoexecCase(t *testing.T, filter Filter) {
	logPath, err := setupTestLog(t, `5027  read(3, "\177ELF\1\1\1\0\0\0\0\0\0\0\0\0\3\0(\0\1\0\0\0\0\0\0\0004\0\0\0"..., 512) = 512
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
		t.Fatal("setupTestLog() failed: ", err)
	}
	defer func() {
		if err := os.Remove(logPath); err != nil {
			t.Errorf("Remove(%q) failed: %v", logPath, err)
		}
	}()

	gen := NewPolicyGenerator()
	if err := gen.AddStraceLog(logPath, filter); err != nil {
		t.Fatal("ApplyResultToPolicyGenerator() failed: ", err)
	}

	type expect struct {
		syscall string // System call.
		freq    int    // Frequency.
		policy  string // Policy.
	}
	expectations := []expect{
		{syscall: "mmap2", freq: 1, policy: "mmap2: arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE"},
		{syscall: "ioctl", freq: 3, policy: "ioctl: arg1 == SIOCGIFFLAGS || arg1 == SIOCSIFFLAGS || arg1 == TCGETS"},
		{syscall: "mprotect", freq: 1, policy: "mprotect: arg2 == PROT_READ|PROT_WRITE|PROT_EXEC"},
	}

	switch filter {
	case IncludeAllSyscalls:
		expectations = append(expectations,
			expect{syscall: "read", freq: 3, policy: "read: 1"},
			expect{syscall: "prctl", freq: 1, policy: "prctl: arg0 == PR_SET_NO_NEW_PRIVS"})
	case ExcludeSyscallsBeforeSandboxing:
		expectations = append(expectations,
			expect{syscall: "read", freq: 1, policy: "read: 1"},
			expect{syscall: "prctl", freq: 0, policy: ""})
	}

	for _, e := range expectations {
		f, p := gen.LookupSyscall(e.syscall)
		if f != e.freq {
			t.Errorf("LookupSyscall(%s) got %d; want %d.", e.syscall, f, e.freq)
			continue
		}
		if p != e.policy {
			t.Errorf("LookupSyscall(%s) got %q; want %q.", e.syscall, p, e.policy)
			continue
		}
	}
}

func testAddStraceLogExecCase(ctx context.Context, t *testing.T, filter Filter) {
	logPath, err := setupTestLog(t, "")
	if err != nil {
		t.Fatal("setupTestLog() failed: ", err)
	}
	defer func() {
		if err := os.Remove(logPath); err != nil {
			t.Errorf("Remove(%q) failed: %v", logPath, err)
		}
	}()

	cmd := CommandContext(ctx, logPath, "/sbin/minijail0", "-n", "/bin/echo", "test string")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		t.Fatalf("%q %q failed with %v.", cmd.Path, cmd.Args, err)
	}

	gen := NewPolicyGenerator()
	if err := gen.AddStraceLog(logPath, filter); err != nil {
		t.Fatal("ApplyResultToPolicyGenerator() failed", err)
	}

	const write = "write"
	expected := "write: 1"
	f, l := gen.LookupSyscall(write)
	if f == 0 {
		t.Errorf("LookupSyscall(%s) failed", write)
	}
	if l != expected {
		t.Errorf("LookupSyscall(%s) got %q; want %q", write, l, expected)
	}

	const prctl = "prctl"
	f, l = gen.LookupSyscall(prctl)
	switch filter {
	case IncludeAllSyscalls:
		if f == 0 {
			t.Errorf("LookupSyscall(%s) unexpectedly failed", prctl)
		}
	case ExcludeSyscallsBeforeSandboxing:
		if f != 0 {
			t.Errorf("LookupSyscall(%s) unexpectedly succeeded with a frequency of %d and value %q", prctl, f, l)
		}
	}
}

func TestPolicyGeneratorAddStraceLog(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testAddStraceLogNoexecCase(t, IncludeAllSyscalls)
	testAddStraceLogNoexecCase(t, ExcludeSyscallsBeforeSandboxing)
	testAddStraceLogExecCase(ctx, t, IncludeAllSyscalls)
	testAddStraceLogExecCase(ctx, t, ExcludeSyscallsBeforeSandboxing)

}

func TestPolicyGeneratorLookupSyscall(t *testing.T) {
	input := []struct {
		syscall string // System call.
		params  string // Parameters.
		lookup  string // Lookup result.
	}{
		{syscall: "execve", params: `/bin/echo", ["echo", "test"], 0x7fff3c1dc228 /* 49 vars */`, lookup: "execve: 1"},
		{syscall: "brk", params: "NULL", lookup: "brk: 1"},
		{syscall: "access", params: `"/etc/ld.so.preload", R_OK`, lookup: "access: 1"},
		{syscall: "openat", params: `AT_FDCWD, "/etc/ld.so.cache", O_RDONLY|O_CLOEXEC`, lookup: "openat: 1"},
		{syscall: "fstat", params: "3, {st_mode=S_IFREG|0644, st_size=62736, ...}", lookup: "fstat: 1"},
		{syscall: "mmap", params: "NULL, 62736, PROT_READ, MAP_PRIVATE, 3, 0", lookup: "mmap: arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE"},
		{syscall: "mmap2", params: "NULL, 62736, PROT_WRITE|PROT_EXEC, MAP_PRIVATE, 3, 0", lookup: "mmap2: arg2 == PROT_WRITE|PROT_EXEC"},
		{syscall: "close", params: "3", lookup: "close: 1"},
		{syscall: "read", params: `3, "\177ELF\2\1\1\3\0\0\0\0\0\0\0\0\3\0>\0\1\0\0\0\260\33\2\0\0\0\0\0"..., 832`, lookup: "read: 1"},
		{syscall: "mprotect", params: "0x7f86fb98d000, 2097152, PROT_NONE", lookup: "mprotect: arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE"},
		{syscall: "arch_prctl", params: "ARCH_SET_FS, 0x7f86fbdad540", lookup: "arch_prctl: 1"},
		{syscall: "munmap", params: "0x7f86fbdae000, 62736", lookup: "munmap: 1"},
		{syscall: "write", params: `1, "test\n", 5`, lookup: "write: 1"},
		{syscall: "ioctl", params: "0, TCGETS, 0xffeb2a18", lookup: "ioctl: arg1 == TCGETS"},
		{syscall: "exit_group", params: "0", lookup: "exit_group: 1"},
	}

	gen := NewPolicyGenerator()
	for i, args := range input {
		gen.AddSyscall(args.syscall, args.params)

		// For even indexed system calls, add them twice to make sure they are counted properly.
		if i%2 == 0 {
			gen.AddSyscall(args.syscall, args.params)
		}
	}

	for i, args := range input {
		expected := 1
		if i%2 == 0 {
			expected = 2
		}
		f, l := gen.LookupSyscall(args.syscall)
		if f != expected {
			t.Errorf("LookupSyscall(%s) failed: got %d; want %d", args.syscall, f, expected)
		}
		if l != args.lookup {
			t.Errorf("LookupSyscall(%s) failed: got %q; want %q", args.syscall, l, args.lookup)
		}
	}
}

func TestPolicyGeneratorGeneratePolicy(t *testing.T) {
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
exit_group: 1
read: 1
restart_syscall: 1
rt_sigreturn: 1
write: 1
`
	actual := gen.GeneratePolicy()
	if actual != expected {
		t.Errorf("GeneratePolicy() failed: got\n%q\nwant\n%q", actual, expected)
	}
}
