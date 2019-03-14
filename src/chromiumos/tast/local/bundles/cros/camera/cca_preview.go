// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAPreview,
		Desc:         "Opens CCA and verifies the preview streams for 3 seconds",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func CCAPreview(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// TODO(shik): Use cr.TestAPIConn once it can use chrome.management.launchApp.
	settings, err := cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to open settings: ", err)
	}
	defer settings.Close()

	const ccaID = "hfhhnacclhffhdffklopdkcgdhifgngh"
	code := fmt.Sprintf(`new Promise((resolve) => chrome.management.launchApp(%q, resolve));`, ccaID)
	if err := settings.EvalPromise(ctx, code, nil); err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}

	// TODO(shik): Unknown race, if we connect too fast then the window will disappear.
	select {
	case <-time.After(time.Second):
	case <-ctx.Done():
		s.Fatal("Timed out while sleeping before connecting to CCA")
	}

	ccaURL := fmt.Sprintf("chrome-extension://%s/views/main.html", ccaID)
	cca, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(ccaURL))
	if err != nil {
		s.Fatal("Failed to connect to CCA: ", err)
	}
	defer cca.Close()
	s.Log("Connected to CCA")

	const isVideoActive = "document.querySelector('video').srcObject.active"
	if err := cca.WaitForExpr(ctx, isVideoActive); err != nil {
		s.Fatal("Failed to start preview: ", err)
	}
	s.Log("Preview started")

	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		s.Fatal("Timed out while streaming preview")
	}

	if err := cca.WaitForExpr(ctx, isVideoActive); err != nil {
		s.Fatal("Preview stopped unexpectedly: ", err)
	}
	s.Log("Preview successfully streamed for 3 seconds")
}
