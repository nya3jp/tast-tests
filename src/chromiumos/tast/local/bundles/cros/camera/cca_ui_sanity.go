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
	"chromiumos/tast/testing/hwdep"
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
			// TODO(crbug.com/1050732): Remove this once the unknown crash on
			// scarlet is resolved.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("scarlet")),
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

	if err := cca.ClearSavedDir(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)
	defer (func() {
		if err := app.CheckJSError(ctx, s.OutDir()); err != nil {
			s.Error("Failed with javascript errors: ", err)
		}
	})()
}
