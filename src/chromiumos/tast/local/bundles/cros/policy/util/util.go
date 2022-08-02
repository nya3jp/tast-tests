// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import "chromiumos/tast/testing"

const pciKey = "pci"

// PolicyTestTag describes the available policy test tags.
type PolicyTestTag string

// Valid values for PolicyTestTag.
const (
	Served     PolicyTestTag = "served"
	Verified   PolicyTestTag = "verified"
	OSVerified PolicyTestTag = "os_verified"
	JSVerified PolicyTestTag = "js_verified"
	UIVerified PolicyTestTag = "ui_verified"
)

// String retrieves the string value of a PolicyTestTag.
func (tag PolicyTestTag) String() string {
	return string(tag)
}

// PCISearchFlags generates SearchFlags based on the given list of PolicyTestTags.
func PCISearchFlags(tags ...PolicyTestTag) []*testing.StringPair {
	stringPairs := make([]*testing.StringPair, len(tags))
	for i, tag := range tags {
		stringPairs[i] = &testing.StringPair{Key: pciKey, Value: tag.String()}
	}

	return stringPairs
}
