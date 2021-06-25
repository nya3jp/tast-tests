// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/common/global"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AccessVars,
		Desc:     "Access variables",
		Contacts: []string{"tast-owners@google.com", "seewaifu@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func AccessVars(ctx context.Context, s *testing.State) {
	if strVal, ok := global.ExampleStrVar.Value(); strVal != "test" || !ok {
		s.Errorf("Got global variable value (%q, %v) from ContextVar want (%q, %v)", strVal, ok, "test", true)
	}
}
