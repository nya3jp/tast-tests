// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpr

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/testing"
)

// NewLacrosFixture creates a new fixture that can launch Lacros chrome with the given setup mode,
// Chrome options, and WPR archive. This should be the child of a WPR fixture.
func NewLacrosFixture(mode launcher.SetupMode, fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return launcher.NewComposedFixture(mode, func(v launcher.FixtValue, pv interface{}) interface{} {
		return v
	}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		opts, err := s.ParentValue().(FixtValue).FOpt()(ctx, s)
		if err != nil {
			return nil, err
		}

		optsExtra, err := fOpt(ctx, s)
		if err != nil {
			return nil, err
		}

		opts = append(opts, optsExtra...)
		return opts, nil
	})
}
