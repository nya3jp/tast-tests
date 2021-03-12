// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

const (
	// TwoRoomSize creates a conference room with 2 participants
	TwoRoomSize = 2
	// SmallRoomSize creates a conference room with 5 participants
	SmallRoomSize = 5
	// LargeRoomSize creates a conference room with 17 participants
	LargeRoomSize = 17
	// ClassRoomSize creates a conference room with 38 participants
	ClassRoomSize = 38
)

// TestParameters defines the test parameters for conference.
type TestParameters struct {
	// Size is the conf room size.
	Size int
	// Tier defines the test tier: basic, plus, or premium.
	Tier string
}
