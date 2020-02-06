// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

type ccaUISanityParams struct {
	useFakeDeviceInChrome bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUISanity,
		Desc:         "Sanity test for Chrome Camera App",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"cca_ui.js"},
		Params: []testing.Param{{
			Name:              "real",
			ExtraSoftwareDeps: []string{caps.BuiltinCamera},
			Pre:               chrome.LoggedIn(),
			Val:               ccaUISanityParams{},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "vivid",
			ExtraSoftwareDeps: []string{caps.VividCamera},
			Pre:               chrome.LoggedIn(),
			Val:               ccaUISanityParams{},
			ExtraAttr:         []string{"informational"},
		}, {
			Name: "fake",
			Val: ccaUISanityParams{
				useFakeDeviceInChrome: true,
			},
		}},
	})
}

func CCAUISanity(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome

	if s.Param().(ccaUISanityParams).useFakeDeviceInChrome {
		var err error
		cr, err = chrome.New(ctx, chrome.ExtraArgs(
			"--use-fake-ui-for-media-stream",
			// The default fps of fake device is 20, but CCA requires fps >= 24.
			// Set the fps to 30 to avoid OverconstrainedError.
			"--use-fake-device-for-media-stream=fps=30"))
		if err != nil {
			s.Fatal("Failed to open chrome: ", err)
		}
		defer cr.Close(ctx)
	} else {
		cr = s.PreValue().(*chrome.Chrome)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching App: ", err)
	}
}
