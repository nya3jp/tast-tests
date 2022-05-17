// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellularlabels

import (
	"context"
	"encoding/json"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// PrintHostInfoLabels prints the host info labels
func PrintHostInfoLabels(ctx context.Context, labels []string) {
	for _, label := range labels {
		testing.ContextLog(ctx, "Labels:", label)
	}
}

// GetHostInfoLabels gets the labels from autotest_host_info_labels var.
func GetHostInfoLabels(ctx context.Context, s *testing.State) ([]string, error) {
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		return nil, errors.New("no labels")
	}

	var labels []string
	if err := json.Unmarshal([]byte(labelsStr), &labels); err != nil {
		return nil, err
	}
	return labels, nil
}
