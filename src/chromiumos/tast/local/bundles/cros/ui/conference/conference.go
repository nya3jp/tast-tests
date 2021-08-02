// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
)

const (
	// TwoRoomSize creates a conference room with 2 participants.
	TwoRoomSize = 2
	// SmallRoomSize creates a conference room with 5 participants.
	SmallRoomSize = 5
	// LargeRoomSize creates a conference room with 16 participants.
	LargeRoomSize = 16
	// ClassRoomSize creates a conference room with 38 participants.
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

const participantError = "number of participants is incorrect (ERROR - PARTICIPANT NUMBER)"

// ParticipantError wraps the given error with participant error specific information
// which can be used to identify the error type with IsParticipantError() function.
func ParticipantError(err error) error {
	return errors.Wrap(err, participantError)
}

// IsParticipantError returns true if the given error contains participant error specific information.
func IsParticipantError(err error) bool {
	// Use string comparason because error loses its type after wrapping.
	return strings.Contains(err.Error(), participantError)
}
