// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAAPI,
		Desc:         "Verifies that the private JavaScript APIs CCA relies on work as expected",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_api_can_access_external_storage.js"},
	})
}

// CCAAPI verifies whether the private JavaScript APIs CCA (Chrome camera app) relies on work as
// expected. The APIs under testing are not owned by CCA team. This test prevents changes to those
// APIs' implementations from silently breaking CCA.
func CCAAPI(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	const ccaID = "hfhhnacclhffhdffklopdkcgdhifgngh"
	bgURL := fmt.Sprintf("chrome-extension://%s/views/background.html", ccaID)
	s.Log("Connecting to CCA background ", bgURL)
	ccaConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatal("Failed to connect to CCA: ", err)
	}
	defer ccaConn.Close()
	defer ccaConn.CloseTarget(ctx)

	rctx, rcancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer rcancel()
	if err := ccaConn.WaitForExpr(rctx, "chrome.fileManagerPrivate"); err != nil {
		s.Fatal("Failed to wait for expression: ", err)
	}
	s.Log("Connected to CCA background")

	testCanAccessExternalStorage(rctx, s, ccaConn)
	// TODO(inker): Add tests for other private APIs.
}

func testCanAccessExternalStorage(ctx context.Context, s *testing.State, conn *chrome.Conn) {
	content, err := ioutil.ReadFile(s.DataPath("cca_api_can_access_external_storage.js"))
	if err != nil {
		s.Error("Failed to read JS file: ", err)
		return
	}
	if err := conn.Eval(ctx, string(content), nil); err != nil {
		s.Error("Failed to evaluate promise: ", err)
	}
}
