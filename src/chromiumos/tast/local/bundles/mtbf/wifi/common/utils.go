// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/testing"
)

// GetVar gets variable from configution file
func GetVar(ctx context.Context, s *testing.State, varName string) string {
	value, ok := s.Var(varName)

	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, varName))
	}

	return value
}
