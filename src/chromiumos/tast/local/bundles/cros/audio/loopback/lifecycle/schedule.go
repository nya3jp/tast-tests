// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lifecycle

import (
	"context"
	"fmt"
)

type schedule struct {
	startSec int
	endSec   int
}

var _ fmt.Stringer = schedule{}

func (s schedule) getSchedule() schedule {
	return s
}

func (s schedule) durationSec() int {
	return s.endSec - s.startSec
}

func (s schedule) add(deltaSec int) schedule {
	return schedule{
		s.startSec + deltaSec,
		s.endSec + deltaSec,
	}
}

func (s schedule) String() string {
	return fmt.Sprintf("(%d,%d)", s.startSec, s.endSec)
}

type timeliner interface {
	maybeLogSchedule(ctx context.Context, t *tester)
	getSchedule() schedule
	durationSec() int
}
