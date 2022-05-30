// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package restrictionlevel contains the different restriction levels used in DLP policies.
package restrictionlevel

// RestrictionLevel is an enum containing the different types of DLP restrictions enforced, potentially including the user's response to the warning dialog (proceed or cancel).
type RestrictionLevel int

// See comment on the type above.
const (
	Allowed RestrictionLevel = iota
	Blocked
	WarnCancelled
	WarnProceeded
)
