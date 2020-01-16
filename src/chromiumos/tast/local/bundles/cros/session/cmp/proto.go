// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cmp provides compare utilities.
package cmp

import (
	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
)

// ProtoDiff returns diff between two proto messages.
func ProtoDiff(a, b proto.Message) string {
	// Due to github.com/golang/protobuf#compatibility, proto structs can contain
	// some system fields that start with XXX_ and we shouldn't compare them.
	// proto.Equal ignores XXX_* fields, so we use it before cmp.Diff to check
	// whether proto structures are equal.
	// TODO(crbug.com/1040909): use diff+protocmp for compare protobufs.
	if !proto.Equal(a, b) {
		// Verify that there's no diff between sent data and fetched data.
		return cmp.Diff(a, b)
	}
	return ""
}
