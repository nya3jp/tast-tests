// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/spera/enterprisecuj"
	cx "chromiumos/tast/local/bundles/cros/spera/enterprisecuj/citrix"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ClinicianWorkstationCUJ,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measure the performance of simulated clinician workstation operations on the Citrix Workspace client app",
		Contacts:     []string{"xliu@cienet.com", "jane.yang@cienet.com"},
		Vars: []string{
			// Optional. Expecting "tablet" or "clamshell". Other values will be be taken as "clamshell".
			"spera.cuj_mode",
			// Required. Credentials used to login Citrix.
			"spera.citrix_url",
			"spera.citrix_username",
			"spera.citrix_password",
			"spera.citrix_desktopname",
			// Required. Used for UI detection API.
			"uidetection.key_type",
			"uidetection.key",
			"uidetection.server",
		},
		Params: []testing.Param{
			{
				Name:    "basic",
				Fixture: "enrolledLoggedInToCUJUser",
				Timeout: 10 * time.Minute,
				Val:     cx.NormalMode,
			},
			{
				// basic_record is a subcase for recording clinician workstation CUJ.
				// When executed, it will record the coordinates and waiting time of all pictures and text
				// detected by uidetection, and will read these data in replay mode.
				Name:    "basic_record",
				Fixture: "enrolledLoggedInToCUJUser",
				Timeout: 10 * time.Minute,
				Val:     cx.RecordMode,
			},
			{
				// basic_replay is a subcase for replaying clinician workstation CUJ.
				// When executed, the coordinates and waiting time of the picture/text recorded in the record
				// mode will be loaded. Use this coordinate data to perform ui click, and reduce this waiting
				// time data to wait for ui. This can greatly reduce the execution time of the case
				Name:    "basic_replay",
				Fixture: "enrolledLoggedInToCUJUser",
				Timeout: 10 * time.Minute,
				Val:     cx.ReplayMode,
			},
		},
		Data: enterprisecuj.ClinicianWorkstationData,
	})
}

func ClinicianWorkstationCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	var tabletMode bool
	if mode, ok := s.Var("spera.cuj_mode"); ok {
		tabletMode = mode == "tablet"
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
		if err != nil {
			s.Fatalf("Failed to enable tablet mode to %v: %v", tabletMode, err)
		}
		defer cleanup(cleanupCtx)
	} else {
		// Use default screen mode of the DUT.
		tabletMode, err = ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get DUT default screen mode: ", err)
		}
	}
	s.Log("Running test with tablet mode: ", tabletMode)
	var uiHandler cuj.UIActionHandler
	if tabletMode {
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to rotate display to landscape: ", err)
		}
		defer cleanup(cleanupCtx)
		if uiHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create tablet action handler: ", err)
		}
	} else {
		if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create clamshell action handler: ", err)
		}
	}

	params := &enterprisecuj.TestParams{
		OutDir:          s.OutDir(),
		CitrixServerURL: s.RequiredVar("spera.citrix_url"),
		CitrixUserName:  s.RequiredVar("spera.citrix_username"),
		CitrixPassword:  s.RequiredVar("spera.citrix_password"),
		DesktopName:     s.RequiredVar("spera.citrix_desktopname"),
		TabletMode:      tabletMode,
		TestMode:        s.Param().(cx.TestMode),
		DataPath:        s.DataPath,
		UIHandler:       uiHandler,
	}
	scenario := enterprisecuj.NewClinicianWorkstationScenario()
	if err := enterprisecuj.Run(ctx, cr, scenario, params); err != nil {
		s.Fatal("Failed to run the clinician workstation cuj: ", err)
	}
}
