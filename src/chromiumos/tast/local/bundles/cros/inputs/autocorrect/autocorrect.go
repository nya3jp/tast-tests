// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package autocorrect contains common defs shared by Autocorrect-related tests.
package autocorrect

import "chromiumos/tast/local/chrome/ime"

// UndoMethod enum corresponds to ways autocorrect can be undone.
type UndoMethod int

// Possible entries of UndoMethod enum.
const (
	ViaBackspace UndoMethod = iota
	ViaPopupUsingPK
	ViaPopupUsingMouse
	NotApplicable
)

// TestCase struct encapsulates parameters for each Autocorrect test.
type TestCase struct {
	InputMethod  ime.InputMethod
	MisspeltWord string
	CorrectWord  string
	UndoMethod   UndoMethod
}
