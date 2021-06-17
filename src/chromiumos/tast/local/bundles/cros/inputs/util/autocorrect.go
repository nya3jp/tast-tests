// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

// AutocorrectUndoMethod enum corresponds to ways autocorrect can be undone.
type AutocorrectUndoMethod int

// Possible entries of AutocorrectUndoMethod enum.
const (
	AutocorrectUndoViaBackspace AutocorrectUndoMethod = iota
	AutocorrectUndoViaPopupUsingPK
	AutocorrectUndoViaPopupUsingMouse
)

// AutocorrectTestCase struct encapsulates parameters for each Autocorrect test.
type AutocorrectTestCase struct {
	InputMethodID string
	MisspeltWord  string
	CorrectWord   string
	UndoMethod    AutocorrectUndoMethod
}
