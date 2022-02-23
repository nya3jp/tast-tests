// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package videovars contains runtime variables used for video testing
package videovars

import (
	"context"
	"strconv"

	"chromiumos/tast/testing"
)

var (
	removeArtifactsVar = testing.RegisterVarString(
		"videovars.removeArtifacts",
		"true",
		"Leave the downloaded/generated artifacts around after the test has run")
)

// ShouldRemoveArtifacts parses removeArtifactsVar to a boolean value.
// If the value of removeArtifactsVar is not supported by strconv.ParseBool(), it will return true.
func ShouldRemoveArtifacts(ctx context.Context) bool {
	removeArtifacts, err := strconv.ParseBool(removeArtifactsVar.Value())

	//If any parse error happens, set the value to true.
	if err != nil {
		testing.ContextLogf(ctx, "Failed to parse video.removeArtifacts value %q, use default value true", removeArtifactsVar.Value())
		removeArtifacts = true
	}
	return removeArtifacts
}
