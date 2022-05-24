// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package toolchain contains support code for the security.ToolchainOptions test.
package toolchain

import (
	"debug/elf"
	"path/filepath"

	"chromiumos/tast/errors"
)

// CheckMode specifies what to check for security.ToolchainOptions.
type CheckMode int

const (
	// CheckNormal tests that files not in allowlists pass checks.
	CheckNormal CheckMode = iota
	// CheckAllowlist tests that files in allowlists fail checks.
	CheckAllowlist
	// CheckNormalWithDLCs tests with critical DLCs installed.
	CheckNormalWithDLCs
)

// ELFCondition is a specific condition which is verified against all
// not-skipped ELF files.
type ELFCondition struct {
	verify    func(ef *elf.File) error
	allowlist []string // list of path patterns to be skipped
}

// NewELFCondition takes a verification function and a list of literal paths
// to allowlist for that condition and returns a new ELFCondition.
func NewELFCondition(verify func(ef *elf.File) error, w []string) *ELFCondition {
	return &ELFCondition{verify, w}
}

// CheckAndFilter takes in a file and checks it against an ELFCondition,
// returning an error if the file is not allowed.
func (ec *ELFCondition) CheckAndFilter(path string, ef *elf.File, mode CheckMode) error {
	allowed := false
	for _, pp := range ec.allowlist {
		matched, err := filepath.Match(pp, path)
		if err != nil {
			return err
		}
		if matched {
			allowed = true
			break
		}
	}

	switch mode {
	case CheckNormal, CheckNormalWithDLCs:
		if allowed {
			return nil
		}
		if err := ec.verify(ef); err != nil {
			return errors.Wrap(err, path)
		}
		return nil
	case CheckAllowlist:
		if !allowed {
			return nil
		}
		if err := ec.verify(ef); err == nil {
			return errors.Wrap(errors.New("allowlist file passed check unexpectedly"), path)
		}
		return nil
	}
	return errors.Errorf("unknown mode %v", mode)
}
