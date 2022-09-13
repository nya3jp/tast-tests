// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtcinternals

import "time"

// TimeWithJSLocaleString holds time.Time and implements interfaces
// encoding.TextMarshaler and encoding.TextUnmarshaler based on the
// behavior of JavaScript toLocaleString().
// Note: The JavaScript function toLocaleString() uses local time by
// default, and so TimeWithJSLocaleString unmarshals with the local time
// zone time.Local. TimeWithJSLocaleString is useful for parsing only in
// the same time zone where JavaScript toLocaleString() was called.
type TimeWithJSLocaleString time.Time

// jsLocaleStringLayout is a date/time layout for use with time.Format,
// time.Parse, and time.ParseInLocation, based on the behavior of
// JavaScript toLocaleString().
const jsLocaleStringLayout = "1/2/2006, 3:04:05 PM"

// MarshalText encodes TimeWithJSLocaleString with jsLocaleStringLayout.
func (t TimeWithJSLocaleString) MarshalText() (text []byte, err error) {
	return []byte(time.Time(t).Format(jsLocaleStringLayout)), nil
}

// UnmarshalText decodes TimeWithJSLocaleString with jsLocaleStringLayout
// and time.Local.
func (t *TimeWithJSLocaleString) UnmarshalText(text []byte) error {
	parsed, err := time.ParseInLocation(jsLocaleStringLayout, string(text), time.Local)
	if err != nil {
		return err
	}

	*t = TimeWithJSLocaleString(parsed)
	return nil
}
