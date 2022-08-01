// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyFirstBootSequence,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify first boot sequence",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      10 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

func VerifyFirstBootSequence(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Opening OS Settings app.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch Settings app: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, time.Minute); err != nil {
		s.Fatal("Settings app did not appear in shelf after launch: ", err)
	}

	v, err := chromeproc.Version(ctx)
	if err != nil {
		s.Error("Failed to get Chrome version: ", err)
	}
	version := strings.Join(v, ".")

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)
	aboutCrOSTab := nodewith.NameContaining("About ChromeOS").Role(role.StaticText)
	if err := ui.DoDefault(aboutCrOSTab)(ctx); err != nil {
		s.Fatal("Failed to click About ChromeOS tab: ", err)
	}

	versionRegex := nodewith.NameRegex(regexp.MustCompile("Version *")).Role(role.StaticText)
	info, err := ui.Info(ctx, versionRegex)
	if err != nil {
		s.Fatal("Failed to get node info: ", err)
	}

	if !strings.Contains(info.Name, version) {
		s.Fatalf("Failed to verify CrOS version: got %q, want %q", info.Name, version)
	}

	ecVersion, err := testexec.CommandContext(ctx, "ectool", "version").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get ectool version: ", err)
	}
	s.Logf("EC Version: %s", ecVersion)

	cbVersion, err := testexec.CommandContext(ctx, "crossystem", "fwid").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get coreboot version: ", err)
	}
	s.Logf("CB Version: %s", cbVersion)

}
