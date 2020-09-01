// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUISmoke,
		Desc:         "Smoke test for Chrome Camera App",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"cca_ui.js"},
		Params: []testing.Param{{
			Name:              "real",
			ExtraSoftwareDeps: []string{caps.BuiltinCamera},
			Pre:               cca.NewPrecondition(cca.ChromeConfig{}),
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "vivid",
			ExtraSoftwareDeps: []string{caps.VividCamera},
			Pre:               cca.NewPrecondition(cca.ChromeConfig{}),
			ExtraAttr:         []string{"informational"},
		}, {
			Name: "fake",
			Pre: cca.NewPrecondition(cca.ChromeConfig{
				UseFakeCamera: true,
			}),
			// TODO(crbug.com/1050732): Remove this once the unknown crash on
			// scarlet is resolved.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("scarlet")),
		}, {
			Name:              "real_swa",
			ExtraSoftwareDeps: []string{caps.BuiltinCamera},
			Pre: cca.NewPrecondition(cca.ChromeConfig{
				InstallSWA: true,
			}),
			ExtraAttr: []string{"informational"},
		}, {
			Name:              "vivid_swa",
			ExtraSoftwareDeps: []string{caps.VividCamera},
			Pre: cca.NewPrecondition(cca.ChromeConfig{
				InstallSWA: true,
			}),
			ExtraAttr: []string{"informational"},
		}, {
			Name: "fake_swa",
			Pre: cca.NewPrecondition(cca.ChromeConfig{
				UseFakeCamera: true,
				InstallSWA:    true,
			}),
			ExtraAttr: []string{"informational"},
			// TODO(crbug.com/1050732): Remove this once the unknown crash on
			// scarlet is resolved.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("scarlet")),
		}},
	})
}

func CCAUISmoke(ctx context.Context, s *testing.State) {
	env := s.PreValue().(*cca.TestEnvironment)
	cr := env.Chrome

	if err := cca.ClearSavedDir(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, env, []string{s.DataPath("cca_ui.js")}, s.OutDir())
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
