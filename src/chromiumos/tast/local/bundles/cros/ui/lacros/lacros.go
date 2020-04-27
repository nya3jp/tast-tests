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

// Setup runs lacros-chrome if indicated by the given ChromeType and returns some objects and interfaces
// useful in tests. If the ChromeType is ChromeTypeLacros, it will return a non-nil LacrosChrome instance or an error.
// If the ChromeType is ChromeTypeChromeOS it will return a nil LacrosChrome instance.
func Setup(ctx context.Context, pd interface{}, crt lacros.ChromeType) (*chrome.Chrome, *launcher.LacrosChrome, ash.ConnSource, error) {
	switch crt {
	case lacros.ChromeTypeChromeOS:
		return pd.(*chrome.Chrome), nil, pd.(*chrome.Chrome), nil
	case lacros.ChromeTypeLacros:
		pd := pd.(launcher.PreData)
		l, err := launcher.LaunchLacrosChrome(ctx, pd)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		return pd.Chrome, l, l, nil
	default:
		return nil, nil, nil, errors.Errorf("unrecognized Chrome type %s", string(crt))
	}
}

// CloseLacrosChrome closes the given lacros-chrome, if it is non-nil. Otherwise, it does nothing.
func CloseLacrosChrome(ctx context.Context, l *launcher.LacrosChrome) {
	if l != nil {
		l.Close(ctx) // Ignore error.
	}
}
