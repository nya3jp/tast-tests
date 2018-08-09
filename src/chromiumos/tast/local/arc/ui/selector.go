// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

// selector holds UI element selection criteria.
//
// This object corresponds to UiSelector in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector
type selector struct {
	Text       string `json:"text,omitempty"`
	ResourceID string `json:"resourceId,omitempty"`

	Mask uint32 `json:"mask"`
}

type SelectorOption func(s *selector)

// newSelector is called from NewObject to construct selector.
func newSelector(opts []SelectorOption) *selector {
	s := &selector{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Text limits the selection criteria by text property.
//
// This corresponds to UiSelector.text in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#text(java.lang.String)
func Text(text string) SelectorOption {
	return func(s *selector) {
		s.Text = text
		s.Mask |= 0x1
	}
}

// ID limits the selection criteria by resource ID.
//
// This corresponds to UiSelector.resourceId in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#resourceId(java.lang.String)
func ID(resourceID string) SelectorOption {
	return func(s *selector) {
		s.ResourceID = resourceID
		s.Mask |= 0x200000
	}
}
