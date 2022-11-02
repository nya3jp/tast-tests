// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ui provides common functions used for UI tests.
package ui

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/ui"
)

// CheckNodeWithNameExists checks if a node containing |name| exist, returns an error is node is not found.
func CheckNodeWithNameExists(ctx context.Context, uiautoSvc ui.AutomationServiceClient, name string) error {
	finder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_NameContaining{NameContaining: name}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(
		ctx, &ui.WaitUntilExistsRequest{Finder: finder}); err != nil {
		return errors.Wrapf(err, "failed to find node with name %s", name)
	}
	return nil
}
