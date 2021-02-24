// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/ghostscript"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     GstorasterUnembeddedFont,
		Desc:     "Tests that the gstoraster CUPS filter handles unembedded PDF fonts",
		Contacts: []string{"batrapranav@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data:         []string{fontFile, fontGoldenFile},
	})
}

const (
	fontFile       = "font-test.pdf"
	fontGoldenFile = "font-golden.pwg"
)

func GstorasterUnembeddedFont(ctx context.Context, s *testing.State) {
	ghostscript.RunTest(ctx, s, "gstoraster", s.DataPath(fontFile), s.DataPath(fontGoldenFile), "" /*envVar*/)
}
