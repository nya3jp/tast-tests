// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// Conference contains user's operation when enter a confernece room.
type Conference interface {
	Join(context.Context, *chrome.TestConn, string) error
	VideoAudioControl(context.Context, *chrome.TestConn) error
	SwitchTabs(context.Context, *chrome.TestConn) error
	ChangeLayout(context.Context, *chrome.TestConn) error
	BackgroundBlurring(context.Context, *chrome.TestConn) error
	ExtendedDisplayPresenting(context.Context, *chrome.TestConn, bool) error
	PresentSlide(context.Context, *chrome.TestConn) error
	StopPresenting(context.Context, *chrome.TestConn) error
	End(context.Context, *chrome.TestConn) error
}
