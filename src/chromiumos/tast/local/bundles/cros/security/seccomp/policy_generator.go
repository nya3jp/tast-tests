// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package seccomp leverages integration tests for generating Minijail seccomp policies.
package seccomp

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// CommandContext wraps the functionality of testexec.CommandContext injecting the strace
// command and arguments writing the strace log to the specified path.
func CommandContext(ctx context.Context, logPath string, name string, arg ...string) *testexec.Cmd {
	cmd := testexec.CommandContext(ctx, "strace", append([]string{"-f", "-o", logPath, "--", name}, arg...)...)
	return cmd
}

type argData struct {
	occurences int                 // Count of times a particular system call has been observed.
	argIndex   int                 // -1 represents the case argIndex and argValues are ignored.
	argValues  map[string]struct{} // The set of observed values of the argIndex-th syscall parameter.
	protFilter bool                // If true, filter the arguments for PROT_EXEC|PROT_WRITE.
}

// PolicyGenerator keeps track of what syscalls have been observed as well as values of a subset of
// arguments for the purpose of generating a Minijail seccomp policy.
//
// It is a reimplementation of
// https://chromium.googlesource.com/aosp/platform/external/minijail/+/master/tools/generate_seccomp_policy.py
// with the goal of not making python a dependency of tast.
type PolicyGenerator struct {
	frequencyData map[string]*argData
}

// NewPolicyGenerator creates an initialized value of PolicyGenerator with sensitive syscalls marked
// so they can be filtered by arguments.
func NewPolicyGenerator() *PolicyGenerator {
	return &PolicyGenerator{map[string]*argData{
		"socket":   {0, 0, map[string]struct{}{}, false}, // int domain
		"ioctl":    {0, 1, map[string]struct{}{}, false}, // int request
		"prctl":    {0, 0, map[string]struct{}{}, false}, // int option
		"mmap":     {0, 2, map[string]struct{}{}, true},  // int prot
		"mmap2":    {0, 2, map[string]struct{}{}, true},  // int prot
		"mprotect": {0, 2, map[string]struct{}{}, true},  // int prot
	}}
}

func (m *PolicyGenerator) addBasicSet() {
	for _, s := range []string{"restart_syscall", "exit", "exit_group", "rt_sigreturn"} {
		if _, ok := m.frequencyData[s]; !ok {
			m.frequencyData[s] = &argData{1, -1, map[string]struct{}{}, false}
		}
	}
}

// AddSyscall records a particular syscall in the frequency data. For sensitive system calls params
// will be parsed so an argument filter can be computed.
func (m *PolicyGenerator) AddSyscall(syscall string, params string) bool {
	entry, ok := m.frequencyData[syscall]
	if !ok {
		m.frequencyData[syscall] = &argData{1, -1, map[string]struct{}{}, false}
		return true
	}

	entry.occurences++

	if entry.argIndex >= 0 {
		tokens := strings.Split(params, ", ")
		if entry.argIndex >= len(tokens) {
			return false
		}
		entry.argValues[strings.TrimSpace(tokens[entry.argIndex])] = struct{}{}
	}
	return true
}

// Filter is an enum for conveying whether or not the target process performs Minijail sandboxing
// for the purpose of ignoring syscalls before the sandbox has been entered.
type Filter int

// See the comment on type Filter.
const (
	IncludeAllSyscalls Filter = iota
	ExcludeSyscallsBeforeSandboxing
)

// AddStraceLog reads the result from the strace log and applies it to the Minijail policy
// generator.
func (m *PolicyGenerator) AddStraceLog(logFile string, filter Filter) (rerr error) {
	h, err := os.Open(logFile)
	if err != nil {
		return errors.Wrapf(err, "open(%q) failed", logFile)
	}
	defer func() {
		if err := h.Close(); err != nil {
			rerr = errors.Wrap(err, "close() failed")
		}
	}()

	sc := bufio.NewScanner(h)
	lines := 0
	type processState int
	const (
		notSandboxed processState = iota
		beingSandboxed
		sandboxed
	)
	f := notSandboxed // Flag to identify when sandboxing has occurred.

	// This extracts the syscall and its parameters from strace output such as:
	// 5028  ioctl(0, TCGETS, {B38400 opost isig icanon echo ...}) = 0
	//       ^^^^^ ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
	//     Group-1 Group-2
	reg := regexp.MustCompile(`^\s*(?:\[[^]]*\]|\d+)?\s*([a-zA-Z0-9_]+)\(([^)<]*)`)
	for sc.Scan() {
		// Parse the line.
		line := strings.TrimSpace(sc.Text())
		g := reg.FindStringSubmatch(line)
		if g == nil {
			// Skip lines that don't match the pattern such as process exit notifications or incomplete
			// system calls which will be repeated when the system call is resumed and completed.
			continue
		}
		syscall, args := g[1], g[2]
		// Sanity check that should never fail.
		if len(syscall) == 0 {
			return errors.New("got empty syscall during parsing")
		}

		// Ignore the first execve since it isn't needed in the seccomp policy.
		lines++
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
	return nil
}

func hasParamWriteAndExec(params *map[string]struct{}) bool {
	for param := range *params {
		tokens := strings.Split(param, "|")
		hasWrite := false
		hasExec := false
		for _, t := range tokens {
			switch strings.TrimSpace(t) {
			case "PROT_WRITE":
				hasWrite = true
			case "PROT_EXEC":
				hasExec = true
			}
		}
		if hasWrite && hasExec {
			return true
		}
	}
	return false
}

// entryToResult formats an entry in frequencyData to a Minijail seccomp policy rule or returns
// {-1, ""} on error.
func entryToResult(syscall string, entry *argData) (int, string) {
	if entry.occurences <= 0 {
		return 0, ""
	}

	if entry.argIndex < 0 {
		return entry.occurences, fmt.Sprintf("%s: 1", syscall)
	}

	// This should never happen.
	if len(entry.argValues) == 0 {
		return -1, ""
	}

	// Memory mapping related system calls generally should not map a region of memory both writable
	// and executable at the same time. When possible, this check replaces the set with
	// "arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE" since it accomplishes the desired intent.
	var argValues []string
	if entry.protFilter && !hasParamWriteAndExec(&entry.argValues) {
		argValues = append(argValues, fmt.Sprintf("arg%d in ~PROT_EXEC", entry.argIndex),
			fmt.Sprintf("arg%d in ~PROT_WRITE", entry.argIndex))
	} else {
		for a := range entry.argValues {
			argValues = append(argValues, fmt.Sprintf("arg%d == %s", entry.argIndex, a))
		}
		sort.Strings(argValues)
	}

	return entry.occurences, fmt.Sprintf("%s: %s", syscall, strings.Join(argValues, " || "))
}

// LookupSyscall gets the frequency count and seccomp policy rule for a system call. If the system
// call isn't found in the frequency data, {0, ""} is returned.
func (m *PolicyGenerator) LookupSyscall(syscall string) (int, string) {
	entry, ok := m.frequencyData[syscall]
	if !ok {
		return 0, ""
	}
	return entryToResult(syscall, entry)
}

// GeneratePolicy returns the Minijail policy that results from the system call frequency data in
// decreasing order of occurrence. Ties are sorted in alphabetical order.
func (m *PolicyGenerator) GeneratePolicy() string {
	m.addBasicSet()

	type ruleWithFrequency struct {
		freq int
		rule string
	}
	var results []*ruleWithFrequency

	for syscall, entry := range m.frequencyData {
		freq, rule := entryToResult(syscall, entry)
		if freq > 0 {
			results = append(results, &ruleWithFrequency{freq: freq, rule: rule})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].freq == results[j].freq {
			return results[i].rule < results[j].rule
		}
		// Descending order.
		return results[i].freq > results[j].freq
	})

	var sb strings.Builder
	for _, e := range results {
		sb.WriteString(e.rule)
		sb.WriteRune('\n')
	}
	return sb.String()
}
