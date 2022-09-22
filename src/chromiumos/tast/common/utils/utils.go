// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils defines utility functions common to local and remote tests.
package utils

import (
	"context"

	"chromiumos/tast/caller"
	"chromiumos/tast/testing"
)

// CollectFirstErr collects the first error into firstErr and logs the others.
// This can be useful when you have several steps in a function but cannot early
// return on error. e.g. cleanup functions.
func CollectFirstErr(ctx context.Context, firstErr *error, err error) {
	if err == nil {
		return
	}
	testing.ContextLogf(ctx, "Error in %s: %s", caller.Get(2), err)
	if *firstErr == nil {
		*firstErr = err
	}
}
