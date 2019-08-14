// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package strace

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/local/testexec"
)

// Cmd is a wrapper around testexec.Cmd for collecting an strace log for generating minijail
// policies. It adds a temporary log file for the output of strace.
type Cmd struct {
	*testexec.Cmd
	straceLog *os.File
}

// CommandContext matches the functionality of testexec.CommandContext, but injects the strace
// command and arguments as well as creating the temp file where the strace log is stored.
func CommandContext(ctx context.Context, name string, arg ...string) *Cmd {
	f, err := ioutil.TempFile("", "tast_strace_")
	if err != nil {
		return nil
	}

	params := []string{"-f", "-o", f.Name(), "--", name}
	params = append(params, arg...)
	cmd := testexec.CommandContext(ctx, "strace", params...)

	return &Cmd{
		Cmd:       cmd,
		straceLog: f,
	}
}

// Filter is an enum for conveying whether or not the target process performs Minijail sandboxing
// for the purpose of ignoring syscalls before the sandbox has been entered.
type Filter int

// See the comment on type Filter.
const (
	IncludeAllSyscalls              Filter = iota
	ExcludeSyscallsBeforeSandboxing Filter = iota
)

func parseLine(line string) (string, string) {
	reg := regexp.MustCompile(`^\s*(?:\[[^]]*\]|\d+)?\s*([a-zA-Z0-9_]+)\(([^)<]*)`)

	g := reg.FindStringSubmatch(line)
	if g == nil {
		return "", ""
	}
	return g[1], g[2]
}

// ApplyResultToPolicyGenerator reads the result from the strace log and apply it to the Minijail
// policy generator.
func (cmd *Cmd) ApplyResultToPolicyGenerator(m *MinijailPolicyGenerator, filter Filter) {
	defer os.Remove(cmd.straceLog.Name())
	defer cmd.straceLog.Close()

	sc := bufio.NewScanner(cmd.straceLog)
	lines := 0
	const (
		notSandboxed   = iota
		beingSandboxed = iota
		sandboxed      = iota
	)
	f := notSandboxed // Flag to identify when sandboxing has occurred.

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		syscall, args := parseLine(line)
		if len(syscall) == 0 {
			continue
		}

		lines++
		// Ignore the first execve since it isn't needed in the seccomp policy.
		if lines == 1 && syscall == "execve" {
			continue
		}

		// Skip until sandboxing completes.
		if filter == ExcludeSyscallsBeforeSandboxing && f != sandboxed {
			// Minijail sets the GID, UID, and no-new-privileges at the end of sandboxing.
			if syscall == "setgroups" || syscall == "setresgid" || syscall == "setresuid" ||
				(syscall == "prctl" && strings.HasPrefix(args, "PR_SET_NO_NEW_PRIVS")) {
				// Once the final sandboxing syscalls are reached transition to the beingSandboxed state
				if f == notSandboxed {
					f = beingSandboxed
				}
				continue
			} else {
				if f == beingSandboxed {
					f = sandboxed
				} else {
					continue
				}
			}
		}

		m.AddSyscall(syscall, args)
	}
}
