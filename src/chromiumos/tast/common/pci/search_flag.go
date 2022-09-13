// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pci contains common constants and utilities that are used by policy
// tests to define Search Flags.
package pci

import (
	"fmt"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/testing"
)

// SearchFlagKey represents the key for Policy Coverage Insights Search Flags.
const SearchFlagKey = "cros/pci"

// Tag describes the available policy test tags.
type Tag string

// Valid values for Tag.
const (
	Served                  Tag = "served"
	VerifiedValue           Tag = "verified_value"
	VerifiedFunctionalityOS Tag = "verified_functionality_os"
	VerifiedFunctionalityJS Tag = "verified_functionality_js"
	VerifiedFunctionalityUI Tag = "verified_functionality_ui"
)

// String retrieves the string value of a Tag.
func (t Tag) String() string {
	return string(t)
}

// SearchFlagWithName generates a StringPair based on the given policy name and tag.
func SearchFlagWithName(n string, t Tag) *testing.StringPair {
	return &testing.StringPair{
		Key:   SearchFlagKey,
		Value: fmt.Sprintf("%s,%s", n, t.String()),
	}
}

// SearchFlag generates a StringPair based on the given policy and tag.
func SearchFlag(p policy.Policy, t Tag) *testing.StringPair {
	return SearchFlagWithName(p.Name(), t)
}
