// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

const ()

func init() {
	testing.AddTest(&testing.Test{
		Func:         PaperTool,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests D-Bus methods related to PaperTool",
		Contacts: []string{
			"masonwilde@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// PaperTool tests D-bus methods related to debugd's PaperTool.
func PaperTool(ctx context.Context, s *testing.State) {
	dbgd, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd D-Bus service: ", err)
	}

	s.Log("Verify PaperDebugSetCategories method")
	const testCategories = debugd.PaperDebugCategoryPrinting | debugd.PaperDebugCategoryScanning
	if err := testPaperDebugSetCategories(ctx, dbgd, testCategories, testCategories); err != nil {
		s.Error("Failed to verify PaperDebugSetCategories: ", err)
	}

	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("syslog.NewReader failed: ", err)
	}
	defer reader.Close()

	s.Log("Listing Scanners")
	if err := testexec.CommandContext(ctx, "lorgnette_cli", "list").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("lorgnette_cli list failed: ", err)
	}

	if _, err := reader.Wait(ctx, 10*time.Second, func(e *syslog.Entry) bool {
		return e.Program == "lorgnette" && e.Severity == "DEBUG"
	}); err != nil {
		s.Fatal("Lorgnette debug log message not found: ", err)
	}
}

// testPaperDebugSetCategories verifies the PaperDebugSetCategories D-Bus method.
func testPaperDebugSetCategories(ctx context.Context, d *debugd.Debugd, categories, expected debugd.PaperDebugCategories) error {
	if err := d.PaperDebugSetCategories(ctx, categories); err != nil {
		return errors.Wrap(err, "failed to call PaperDebugSetCategories")
	}

	return nil
}
