// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleon

import (
	"context"
	"time"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

// Plug calls the Chameleon plug method.
func (s *Chameleon) Plug(ctx context.Context, portID int) error {
	return s.xmlrpc.Run(servo.NewCall("Plug", portID))
}

// Unplug calls the Chameleon Unplug method.
func (s *Chameleon) Unplug(ctx context.Context, portID int) error {
	return s.xmlrpc.Run(servo.NewCall("Unplug", portID))
}

// WaitVideoInputStable calls the Chameleon WaitVideoInputStable method.
func (s *Chameleon) WaitVideoInputStable(ctx context.Context, portID int, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		return s.xmlrpc.Run(servo.NewCall("WaitVideoInputStable", portID))
	}, &testing.PollOptions{Timeout: timeout})
}
