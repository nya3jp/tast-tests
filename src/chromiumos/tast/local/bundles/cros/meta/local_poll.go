// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalPoll,
		Desc:     "Intentionally fails due to timeout",
		Contacts: []string{"tast-owners@google.com"},
	})
}

func LocalPoll(ctx context.Context, s *testing.State) {
	timeout := time.Second + 2*time.Millisecond + 3*time.Nanosecond
	opts := &testing.PollOptions{
		Timeout:  timeout,
		Interval: time.Millisecond,
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return errors.New("error to force to timeout")
	}, opts); err != nil {
		s.Error("Error with poll: ", err)
	}
}
