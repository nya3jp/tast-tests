// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Hammer,
		Desc: "Runs hammer for 1h",
		Timeout: 2 * time.Hour,
		Contacts: []string{
			"drinkcat@chromium.org",
			"cros-partner-avl@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func RunHammer(ctx context.Context) error {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get out dir")
	}
	f, err := os.Create(filepath.Join(outDir, "hammer.txt"))
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx,
		"/usr/local/hammer",
		"/usr/local/mt8183-kukui-big-quad-1h.cfg")
	if f != nil {
		cmd.Stdout = f
	}
	return cmd.Run(testexec.DumpLogOnError)
}

func Hammer(ctx context.Context, s *testing.State) {
	s.Log("Testing for 1h.")
	if err := RunHammer(ctx); err != nil {
		s.Fatal("hammer failed: ", err)
	}
}
