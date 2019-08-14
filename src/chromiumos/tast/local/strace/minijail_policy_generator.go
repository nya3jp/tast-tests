// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package strace

import (
	"fmt"
	"log/syslog"
	"sort"
	"strings"
)

type argData struct {
	occurences int
	argIndex   int
	argValues  map[string]struct{}
}

// MinijailPolicyGenerator keeps track of what syscalls have been observed as well as values of a
// subset of arguments for the purpose of generating a Minijail seccomp policy.
type MinijailPolicyGenerator struct {
	frequencyData map[string]interface{}
}

// NewMinijailPolicyGenerator creates an initialized value of MinijailPolicyGenerator with sensitive
// syscalls marked so they can be filtered by arguments.
func NewMinijailPolicyGenerator() *MinijailPolicyGenerator {
	return &MinijailPolicyGenerator{map[string]interface{}{
		"socket":   &argData{0, 0, map[string]struct{}{}}, // int domain
		"ioctl":    &argData{0, 1, map[string]struct{}{}}, // int request
		"prctl":    &argData{0, 0, map[string]struct{}{}}, // int option
		"mmap":     &argData{0, 2, map[string]struct{}{}}, // int prot
		"mmap2":    &argData{0, 2, map[string]struct{}{}}, // int prot
		"mprotect": &argData{0, 2, map[string]struct{}{}}, // int prot
	}}
}

func (m *MinijailPolicyGenerator) addBasicSet() {
	for _, s := range []string{"restart_syscall", "exit", "exit_group", "rt_sigreturn"} {
		if _, ok := m.frequencyData[s]; !ok {
			var x int
			x = 1
			m.frequencyData[s] = x
		}
	}
}

// AddSyscall records a particular syscall in the frequency data. For sensitive system calls params
// will be parsed so an argument filter can be computed.
func (m *MinijailPolicyGenerator) AddSyscall(syscall string, params string) {
	entry, ok := m.frequencyData[syscall]
	if !ok {
		var x int
		x = 1
		m.frequencyData[syscall] = x
		return
	}

	if syscall == "ioctl" {
		log, _ := syslog.NewLogger(syslog.LOG_ERR, 0)
		log.Printf("%s args:%s", syscall, params)
	}

	switch entry.(type) {
	case int:
		m.frequencyData[syscall] = entry.(int) + 1
	case *argData:
		d := entry.(*argData)
		d.occurences++
		i := d.argIndex
		tokens := strings.Split(params, ", ")
		if i >= len(tokens) {
			return
		}
		d.argValues[tokens[i]] = struct{}{}
	}
}

func hasParamWriteAndExec(params *map[string]struct{}) bool {
	for param := range *params {
		tokens := strings.Split(param, "|")
		hasWrite := false
		hasExec := false
		for _, t := range tokens {
			clean := strings.TrimSpace(t)
			if clean == "PROT_WRITE" {
				if hasExec {
					return true
				}
				hasWrite = true
			} else if clean == "PROT_EXEC" {
				if hasWrite {
					return true
				}
				hasExec = true
			}
		}
	}
	return false
}

func entryToResult(syscall string, entry interface{}) (int, string) {
	switch entry.(type) {
	case int:
		return entry.(int), fmt.Sprintf("%s: 1", syscall)
	case *argData:
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s: ", syscall))

		d := entry.(*argData)
		i := d.argIndex

		if i == 2 && (syscall == "mmap" || syscall == "mmap2" || syscall == "mprotect") &&
			!hasParamWriteAndExec(&d.argValues) {
			sb.WriteString("arg2 in ~PROT_EXEC || arg2 in ~PROT_WRITE")
			return d.occurences, sb.String()
		}

		if syscall == "ioctl" {
			logger, _ := syslog.NewLogger(syslog.LOG_ALERT, 0)
			logger.Printf("ioctl: %v", d.argValues)
		}

		if len(d.argValues) == 0 {
			return 0, ""
		}

		argValues := []string{}
		for a := range d.argValues {
			argValues = append(argValues, a)
		}

		sort.Slice(argValues, func(i, j int) bool {
			return argValues[i] < argValues[j]
		})

		first := true
		for _, a := range argValues {
			if first {
				first = false
			} else {
				sb.WriteString(" || ")
			}
			sb.WriteString(fmt.Sprintf("arg%d == %s", i, a))
		}

		return d.occurences, sb.String()
	}
	return 0, ""
}

// LookupSyscall gets the frequency count and seccomp policy rule for a system call. If the system
// call isn't found in the frequency data, {0, ""} is returned.
func (m *MinijailPolicyGenerator) LookupSyscall(syscall string) (int, string) {
	entry, ok := m.frequencyData[syscall]
	if !ok {
		return 0, ""
	}
	return entryToResult(syscall, entry)
}

// GeneratePolicy returns the Minijail policy that results from the system call frequency data in
// decreasing order of occurrence. Ties are sorted in alphabetical order.
func (m *MinijailPolicyGenerator) GeneratePolicy() string {
	type ruleWithFrequency struct {
		f int
		r string
	}
	results := []*ruleWithFrequency{}

	for syscall, entry := range m.frequencyData {
		f, r := entryToResult(syscall, entry)
		if f > 0 {
			results = append(results, &ruleWithFrequency{f: f, r: r})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].r < results[j].r
	})
	sort.SliceStable(results, func(i, j int) bool {
		// Use decreasing order.
		return results[i].f > results[j].f
	})

	var sb strings.Builder
	for _, e := range results {
		sb.WriteString(e.r)
		sb.WriteRune('\n')
	}
	return sb.String()
}
