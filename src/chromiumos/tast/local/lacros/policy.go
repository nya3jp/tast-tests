// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros implements a library used for utilities and communication with lacros-chrome on ChromeOS.
package lacros

import (
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/policyutil/fixtures"
)

// GetChrome gets the *chrome.Chrome object given some FixtData, which may be lacros launcher.FixtData.
func GetChrome(f interface{}) (*chrome.Chrome, error) {
	switch f.(type) {
	case *chrome.Chrome:
		return f.(*chrome.Chrome), nil
	case launcher.FixtData:
		return f.(launcher.FixtData).Chrome, nil
	case *fixtures.FixtData:
		return f.(*fixtures.FixtData).Chrome, nil
	default:
		return nil, errors.Errorf("unrecognized FixtValue type: %v", f)
	}
}

// GetFakeDMS gets the *fakedms.FakeDMS object given some FixtData, which may be lacros launcher.FixtData.
func GetFakeDMS(f interface{}) (*fakedms.FakeDMS, error) {
	switch f.(type) {
	case *fakedms.FakeDMS:
		return f.(*fakedms.FakeDMS), nil
	case launcher.FixtData:
		return f.(launcher.FixtData).FakeDMS, nil
	case *fixtures.FixtData:
		return f.(*fixtures.FixtData).FakeDMS, nil
	default:
		return nil, errors.Errorf("unrecognized FixtValue type: %v", f)
	}
}
