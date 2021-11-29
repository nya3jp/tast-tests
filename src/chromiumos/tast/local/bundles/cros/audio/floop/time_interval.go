// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package floop

import "fmt"

// TI is a shorthand to create TimeIntervals
func TI(startSec, endSec int) TimeInterval {
	return TimeInterval{StartSec: startSec, EndSec: endSec}
}

// TimeInterval specifies when a event should happen in the test timeline
type TimeInterval struct {
	StartSec int
	EndSec   int
}

var _ fmt.Stringer = TimeInterval{}

var zeroInterval TimeInterval

// DurationSec returns the length of the TimeInterval in seconds
func (ti TimeInterval) DurationSec() int {
	return ti.EndSec - ti.StartSec
}

func (ti TimeInterval) String() string {
	return fmt.Sprintf("(%d, %d)", ti.StartSec, ti.EndSec)
}
