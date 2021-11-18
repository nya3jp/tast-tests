// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/cujrunner"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CUJRunner,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Run a CUJ json config and measure performance",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      "loggedInToCUJUser",
		Timeout:      30 * time.Minute,
		Data:         []string{"cuj_config.json"},
		Vars: []string{
			"ui.cuj_password",
		},
	})
}

func CUJRunner(ctx context.Context, s *testing.State) {
	runner := cujrunner.NewRunner(s.FixtValue().(cuj.FixtureData).Chrome)
	if err := runner.Run(ctx, s, s.DataPath("cuj_config.json")); err != nil {
		s.Fatal("Failed to run: ", err)
	}
}
