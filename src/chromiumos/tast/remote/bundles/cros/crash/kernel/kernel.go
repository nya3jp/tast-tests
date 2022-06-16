// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kernel provides common functions for the kernel and hypervisor tests.
package kernel

import (
	"io/ioutil"
	"path"
	"path/filepath"
	"regexp"

	"chromiumos/tast/testing"
)

// VerifyMetadata checks that the crash metadata has reasonable values.
func VerifyMetadata(s *testing.State, fname, execName string) {
	execNameRegexp := regexp.MustCompile("(?m)^exec_name=" + execName + "$")
	badSigRegexp := regexp.MustCompile("sig=kernel-.+-00000000")
	goodSigRegexp := regexp.MustCompile("sig=kernel-.+-[[:xdigit:]]{8}")
	f, err := ioutil.ReadFile(filepath.Join(s.OutDir(), path.Base(fname)))
	if err != nil {
		s.Error("Failed to read meta file: ", fname)
		return
	}
	s.Log("Checking exec_name")
	if !execNameRegexp.Match(f) {
		s.Error("Found wrong exec_name in meta file: ", fname)
	}
	s.Log("Checking signature line for non-zero")
	if badSigRegexp.Match(f) {
		s.Error("Found all zero signature in meta file: ", fname)
	} else if !goodSigRegexp.Match(f) {
		s.Error("Couldn't find unique signature in meta file: ", fname)
	}
}
