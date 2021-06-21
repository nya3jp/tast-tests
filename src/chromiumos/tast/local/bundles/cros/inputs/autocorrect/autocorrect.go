// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package autocorrect contains common defs shared by Autocorrect-related tests.
package autocorrect

// UndoMethod enum corresponds to ways autocorrect can be undone.
type UndoMethod int

// Possible entries of UndoMethod enum.
const (
	ViaBackspace UndoMethod = iota
	ViaPopupUsingPK
	ViaPopupUsingMouseOrTouch
)

// TestCase struct encapsulates parameters for each Autocorrect test.
type TestCase struct {
	InputMethodID string
	MisspeltWord  string
	CorrectWord   string
	UndoMethod    UndoMethod
}
