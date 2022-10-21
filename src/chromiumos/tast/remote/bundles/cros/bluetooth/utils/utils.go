// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils contains shared code within bluetooth folder.
package utils

import (
	"context"

	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func checkNodeWithNameExists(ctx context.Context, uiautoSvc ui.AutomationServiceClient, s *testing.State, name string) error {
	finder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_NameContaining{NameContaining: name}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: finder}); err != nil {
		return errors.Wrapf(err, "failed to find %s", name)
	}
	return nil
}
