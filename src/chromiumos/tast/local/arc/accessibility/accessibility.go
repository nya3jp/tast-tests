// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accessibility provides functions to interact with accessibility settings
// over Android and Chrome.
package accessibility

import (
	"context"
	"strings"

	"chromiumos/tast/local/arc"
)

// IsAccessibilityEnabled checks if accessibility is enabled in Android.
func IsAccessibilityEnabled(ctx context.Context, a *arc.ARC) (enabled bool, err error) {
	cmd := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled")
	res, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return false, err
	}
	if strings.TrimSpace(string(res)) == "1" {
		return true, nil
	}
	return false, nil
}
