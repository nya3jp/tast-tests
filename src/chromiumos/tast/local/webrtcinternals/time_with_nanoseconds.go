// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtcinternals

import "time"

// TimeWithNanoseconds holds time.Time and implements interfaces
// encoding.TextMarshaler and encoding.TextUnmarshaler with
// time.RFC3339Nano. Compared to time.Time which implements those
// interfaces with time.RFC3339, the difference is that
// TimeWithNanoseconds includes nanoseconds in its text representation.
type TimeWithNanoseconds time.Time

// MarshalText encodes TimeWithNanoseconds with time.RFC3339Nano.
func (t TimeWithNanoseconds) MarshalText() (text []byte, err error) {
	return []byte(time.Time(t).Format(time.RFC3339Nano)), nil
}

// UnmarshalText decodes TimeWithNanoseconds with time.RFC3339Nano.
func (t *TimeWithNanoseconds) UnmarshalText(text []byte) error {
	parsed, err := time.Parse(time.RFC3339Nano, string(text))
	if err != nil {
		return err
	}

	*t = TimeWithNanoseconds(parsed)
	return nil
}
