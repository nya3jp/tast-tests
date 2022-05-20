// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtcinternals

import "encoding/json"

// SliceWithJSONQuotedString holds a slice and implements interfaces
// encoding.TextMarshaler and encoding.TextUnmarshaler with a JSON array
// representation. SliceWithJSONQuotedString intentionally does not implement
// interfaces json.Marshaler and json.Unmarshaler. The purpose is for dealing
// with JSON that looks like this:
// "values": "[8000,8000,8000,8000]"
type SliceWithJSONQuotedString []interface{}

// MarshalText encodes SliceWithJSONQuotedString as a JSON array.
func (s SliceWithJSONQuotedString) MarshalText() (text []byte, err error) {
	return json.Marshal([]interface{}(s))
}

// UnmarshalText decodes SliceWithJSONQuotedString from a JSON array.
func (s *SliceWithJSONQuotedString) UnmarshalText(text []byte) error {
	return json.Unmarshal(text, (*[]interface{})(s))
}
