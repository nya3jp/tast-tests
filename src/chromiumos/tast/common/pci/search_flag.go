// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pci contains common constants and utilities that are used by policy
// tests to define Search Flags.
package pci

import (
	"fmt"

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
func (tag Tag) String() string {
	return string(tag)
}

// SearchFlagWithPolicyName generates a StringPair based on the given policy name and tag.
func SearchFlagWithPolicyName(policyName string, tag Tag) *testing.StringPair {
	return &testing.StringPair{
		Key:   SearchFlagKey,
		Value: fmt.Sprintf("%s,%s", policyName, tag.String()),
	}
}

// Namer describes a type which has a Name function that returns a string value.
type Namer interface {
	Name() string
}

// SearchFlag generates a StringPair based on the given policy type and tag.
func SearchFlag[N Namer](tag Tag) *testing.StringPair {
	namer := new(N)
	return SearchFlagWithPolicyName((*namer).Name(), tag)
}
