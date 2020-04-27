// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros implements helper utilities for running lacros test variations.
package lacros

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
)

// Setup runs Linux chrome if indicated by the given ChromeType and returns some objects and interfaces
// useful in tests. If the ChromeType is ChromeTypeLacros, it will return a non-nil LinuxChrome instance or an error.
// If the ChromeType is ChromeTypeChromeOS it will return a nil LinuxChrome instance.
func Setup(ctx context.Context, pd interface{}, crt lacros.ChromeType) (*chrome.Chrome, *launcher.LinuxChrome, ash.ConnSource, error) {
	if crt == lacros.ChromeTypeChromeOS {
		return pd.(*chrome.Chrome), nil, pd.(*chrome.Chrome), nil
	} else if crt == lacros.ChromeTypeLacros {
		pd := pd.(launcher.PreData)
		l, err := launcher.LaunchLinuxChrome(ctx, pd)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to launch linux-chrome")
		}
		return pd.Chrome, l, l, nil
	}
	return nil, nil, nil, errors.Errorf("unrecognized Chrome type %s", string(crt))
}

// CloseLinuxChrome closes the given Linux chrome, if it is non-nil. Otherwise, it does nothing.
func CloseLinuxChrome(ctx context.Context, l *launcher.LinuxChrome) {
	if l != nil {
		l.Close(ctx) // Ignore error.
	}
}
