// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/testexec"
)

type localRunner struct{}

func (*localRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return testexec.CommandContext(ctx, name, args...).Output()
}

type LocalClient struct {
	*hwsec.BaseClient
}

func NewLocalClient() *LocalClient {
	return &LocalClient{BaseClient: hwsec.NewBaseClient(&localRunner{})}
}
