// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lifecycle

import (
	"context"

	"chromiumos/tast/testing"
)

// Action is used to specify what to do in Param
type Action interface {
	timeliner
	Do(ctx context.Context, s *testing.State, t *tester)
}

// CaptureAction is a Action which captures audio to tester.captureRaw
type CaptureAction interface {
	Action
	magicIsCaptureAction()
}

// PlaybackAction is a Action which plays audio from tester.playbackRaw
type PlaybackAction interface {
	Action
	magicIsPlaybackAction()
}

// isCaptureAction marks a Action struct as a CaptureAction for type checks
type isCaptureAction struct{}

func (m isCaptureAction) magicIsCaptureAction() {}

// isPlaybackAction marks a Action struct as a PlaybackAction for type checks
type isPlaybackAction struct{}

func (m isPlaybackAction) magicIsPlaybackAction() {}
