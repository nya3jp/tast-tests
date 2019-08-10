// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cpuset implement ARC cpu restriction tests.
package cpuset

import (
	"fmt"
	"io/ioutil"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

// CPUSet represents a set of integers
type CPUSet map[int]struct{}

// Equal returns two only if both CPUSets are identical.
func (a CPUSet) Equal(b CPUSet) bool {
	for ka := range a {
		_, ok := b[ka]
		if !ok {
			return false
		}
		delete(b, ka)
	}

	if len(b) != 0 {
		return false
	}

	return true
}

// StrictSuperset is true only if the passed CPUSet is strictly a subset
// of this CPUSet.
func (a CPUSet) StrictSuperset(b CPUSet) bool {
	// Cannot have something in B not in A
	for kb := range b {
		_, ok := a[kb]
		if !ok {
			return false
		}
	}

	// And A must contain something not in B
	for ka := range a {
		_, ok := b[ka]
		if !ok {
			return true
		}
	}

	// Can't be equal
	return false
}

// Union returns the union of two CPUSets.
func (a CPUSet) Union(b CPUSet) CPUSet {
	r := a
	for k := range b {
		r[k] = struct{}{}
	}
	return r
}

// Online returns a CPUSet with all online cpus at time of call.
func Online() CPUSet {
	out, err := ioutil.ReadFile("/sys/devices/system/cpu/online")
	if err != nil {
		panic(err)
	}
	online, err := Parse(string(out))
	if err != nil {
		panic(err)
	}
	return online
}

// Offline returns a CPUSet with all offline cpus at time of call.
func Offline() CPUSet {
	out, err := ioutil.ReadFile("/sys/devices/system/cpu/offline")
	if err != nil {
		panic(err)
	}
	offline, err := Parse(string(out))
	if err != nil {
		panic(err)
	}
	return offline
}

// Present returns a CPUSet with both online and offline cpus.
func Present() CPUSet {
	return Online().Union(Offline())
}

// Parse parses cpus file content from /dev/cpuset/*/cpus and returns map
// of active CPUs.
func Parse(content string) (map[int]struct{}, error) {
	cpusInUse := make(map[int]struct{})
	for _, subset := range strings.Split(strings.TrimSpace(content), ",") {
		var fromCPU int
		var toCPU int

		if _, err := fmt.Sscanf(subset, "%d-%d", &fromCPU, &toCPU); err == nil {
			for i := fromCPU; i <= toCPU; i++ {
				cpusInUse[i] = struct{}{}
			}
			continue
		}
		if _, err := fmt.Sscanf(subset, "%d", &fromCPU); err == nil {
			cpusInUse[fromCPU] = struct{}{}
			continue
		}
		return nil, errors.Errorf("failed to parse cpus value %q", subset)
	}

	return cpusInUse, nil
}

// CheckCPUSpec iterates over the cgroups specified applying the
// supplied check function.
func CheckCPUSpec(s *testing.State, spec map[string]func(CPUSet) bool) {
	initPID, err := arc.InitPID()
	if err != nil {
		s.Error("Failed to get root init process")
		return
	}
	for k, checkFunc := range spec {
		path := fmt.Sprintf("/proc/%d/root/dev/cpuset/%s/effective_cpus", initPID, k)

		out, err := ioutil.ReadFile(path)
		if err != nil {
			s.Errorf("Failed to read %s: %v", path, err)
			continue
		}
		cpusInUse, err := Parse(string(out))
		if err != nil {
			s.Errorf("Failed to parse %s: %v", path, err)
			continue
		}

		if !checkFunc(cpusInUse) {
			s.Errorf("Unexpected CPU setting for %q from %s: got %q", k, path, strings.TrimSpace(string(out)))
		}
	}
}
