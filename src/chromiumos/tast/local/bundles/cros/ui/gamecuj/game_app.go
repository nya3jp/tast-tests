// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gamecuj

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	defaultUITimeout = 15 * time.Second // Used for situations where UI response might be slow.
	shortUITimeout   = 3 * time.Second  // Used for situations where UI response are faster.
)

// GameApp contains user's operation in game application.
type GameApp interface {
	Install(ctx context.Context) error
	Launch(ctx context.Context) (time.Duration, error)
	End(ctx context.Context) error
	Play(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error
}
