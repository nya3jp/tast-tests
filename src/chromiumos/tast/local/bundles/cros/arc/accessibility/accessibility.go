// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accessibility provides functions to assist with interacting with accessibility settings
// in ARC accessibility tests.
package accessibility

import (
	"context"
	"strings"

	"chromiumos/tast/local/arc"
)

// Enabled checks if accessibility is enabled in Android.
func Enabled(ctx context.Context, a *arc.ARC) (bool, error) {
	cmd := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled")
	res, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return false, err
	}
	return strings.TrimSpace(string(res)) == "1", nil
}
