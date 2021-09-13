// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package jsontypes provides the alias types of int64,uint64,uint32 and the
// json parsers for these types, because json doesn't support these type
// natively.
package jsontypes

import (
	"encoding/json"
	"strconv"
)

// Int64 is an alias of int64
type Int64 int64

// Int32 is an alias of int32
type Int32 int32

// Uint64 is an alias of uint64
type Uint64 uint64

// Uint32 is an alias of uint32
type Uint32 uint32

// String is an alias of string
type String string

// Bool is an alias of bool
type Bool bool

func parseInt64(b []byte) (int64, error) {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return 0, err
	}
	return strconv.ParseInt(s, 10, 64)
}

func parseUint(b []byte, bitSize int) (uint64, error) {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return 0, err
	}
	return strconv.ParseUint(s, 10, bitSize)
}

// UnmarshalJSON Int64 implementation.
func (n *Int64) UnmarshalJSON(b []byte) error {
	x, err := parseInt64(b)
	if err != nil {
		return err
	}

	*n = Int64(x)
	return nil
}

// UnmarshalJSON Uint64 implementation.
func (n *Uint64) UnmarshalJSON(b []byte) error {
	x, err := parseUint(b, 64)
	if err != nil {
		return err
	}

	*n = Uint64(x)
	return nil
}

// UnmarshalJSON Uint32 implementation.
func (n *Uint32) UnmarshalJSON(b []byte) error {
	x, err := parseUint(b, 32)
	if err != nil {
		return err
	}

	*n = Uint32(x)
	return nil
}
