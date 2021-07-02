// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package jsontypes

import (
	"math"
	"strconv"
	"testing"
)

func TestUnmarshalInt64(t *testing.T) {
	var x Int64
	maxInt64 := strconv.Quote(strconv.FormatInt(math.MaxInt64, 10))
	if err := x.UnmarshalJSON([]byte(maxInt64)); err != nil {
		t.Errorf("Fail to unmarshal int64 (%s): %v", maxInt64, err)
	}
	minInt64 := strconv.Quote(strconv.FormatInt(math.MinInt64, 10))
	if err := x.UnmarshalJSON([]byte(minInt64)); err != nil {
		t.Errorf("Fail to unmarshal int64 (%s): %v", minInt64, err)
	}
}

func TestUnmarshalUint64(t *testing.T) {
	var x Uint64
	maxUint64 := strconv.Quote(strconv.FormatUint(math.MaxUint64, 10))
	if err := x.UnmarshalJSON([]byte(maxUint64)); err != nil {
		t.Errorf("Fail to unmarshal uint64 (%s): %v", maxUint64, err)
	}
}

func TestUnmarshalUint32(t *testing.T) {
	var x Uint32
	maxUint32 := strconv.Quote(strconv.FormatUint(math.MaxUint32, 10))
	if err := x.UnmarshalJSON([]byte(maxUint32)); err != nil {
		t.Errorf("Fail to unmarshal uint32 (%s): %v", maxUint32, err)
	}
}
