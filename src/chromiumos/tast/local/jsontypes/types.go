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

// Uint64 is an alias of uint64
type Uint64 uint64

// Uint32 is an alias of uint32
type Uint32 uint32

func parseInt(b []byte, bs int) (int64, error) {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return 0, err
	}
	return strconv.ParseInt(s, 10, bs)
}

func parseUint(b []byte, bs int) (uint64, error) {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return 0, err
	}
	return strconv.ParseUint(s, 10, bs)
}

// UnmarshalJSON Int64 implementation.
func (n *Int64) UnmarshalJSON(b []byte) error {
	var err error
	x, err := parseInt(b, 64)
	if err != nil {
		return err
	}

	*n = Int64(x)
	return nil
}

// UnmarshalJSON Uint64 implementation.
func (n *Uint64) UnmarshalJSON(b []byte) error {
	var err error
	x, err := parseUint(b, 64)
	if err != nil {
		return err
	}

	*n = Uint64(x)
	return nil
}

// UnmarshalJSON Uint32 implementation.
func (n *Uint32) UnmarshalJSON(b []byte) error {
	var err error
	x, err := parseUint(b, 32)
	if err != nil {
		return err
	}

	*n = Uint32(x)
	return nil
}

func formatInt(n int64) ([]byte, error) {
	s := strconv.FormatInt(n, 10)
	return json.Marshal(s)
}

func formatUint(n uint64) ([]byte, error) {
	s := strconv.FormatUint(n, 10)
	return json.Marshal(s)
}
