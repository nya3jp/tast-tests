// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
)

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

// Conference contains user's operation when enter a confernece room.
type Conference interface {
	Join(ctx context.Context, room string) error
	VideoAudioControl(ctx context.Context) error
	SwitchTabs(ctx context.Context) error
	ChangeLayout(ctx context.Context) error
	BackgroundBlurring(ctx context.Context) error
	ExtendedDisplayPresenting(ctx context.Context) error
	PresentSlide(ctx context.Context) error
	StopPresenting(ctx context.Context) error
	End(ctx context.Context) error
}
