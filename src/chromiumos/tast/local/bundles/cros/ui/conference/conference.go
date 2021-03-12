// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
)

// Conference contains user's operation when enter a confernece room.
type Conference interface {
	// Join joins the conference.
	Join(ctx context.Context, room string) error
	VideoAudioControl(ctx context.Context) error
	SwitchTabs(ctx context.Context) error
	ChangeLayout(ctx context.Context) error
	BackgroundBlurring(ctx context.Context) error
	ExtendedDisplayPresenting(ctx context.Context, tabletMode bool) error
	PresentSlide(ctx context.Context) error
	StopPresenting(ctx context.Context) error
	End(ctx context.Context) error
}
