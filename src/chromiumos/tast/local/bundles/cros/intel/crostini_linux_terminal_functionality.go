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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniLinuxTerminalFunctionality,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies crostini linux terminal installation and test VT-d functionality",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{crostini.GetContainerMetadataArtifact("buster", false), crostini.GetContainerRootfsArtifact("buster", false)},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{
			{
				Name:    "only_install",
				Val:     false,
				Timeout: 5 * time.Minute,
			}, {
				Name:    "install_and_check_vtd",
				Val:     true,
				Timeout: 5 * time.Minute,
			},
		},
	})
}

func CrostiniLinuxTerminalFunctionality(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	isVTDFunctionalCheck := s.Param().(bool)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Install Crostini.
	iOptionsUser := crostini.GetInstallerOptions(s, vm.DebianBuster, false /*largeContainer*/, cr.NormalizedUser())
	if _, err := ui.InstallCrostini(ctx, tconn, cr, iOptionsUser); err != nil {
		s.Fatal("Failed to install Crostini for user: ", err)
	}

	// After successful Crostini terminal installation, close all opened windows.
	if err := cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset Chrome state: ", err)
	}

	terminalApp, err := terminalapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch terminal app after installing Crostini: ", err)
	}
	defer terminalApp.Close()(cleanupCtx)

	if err := terminalApp.WaitForPrompt()(ctx); err != nil {
		s.Fatal("Failed to wait for prompt: ", err)
	}

	cmdOutput := func(cmd string) string {
		out, err := testexec.CommandContext(ctx, "bash", "-c", cmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute %q command: %v", cmd, err)
		}
		return string(out)
	}

	if !isVTDFunctionalCheck {
		return
	}

	const (
		lscpuCommand   = "lscpu | grep VT"
		cmdLineCommand = "cat /proc/cmdline"
		dmesgCommand   = "dmesg | grep DMAR"
	)

	lscpuOut := cmdOutput(lscpuCommand)
	lscpuRe := regexp.MustCompile(`Virtualization.*VT-x`)
	if !lscpuRe.MatchString(lscpuOut) {
		s.Fatalf("Failed to get virtualization VT info: got %q , want match %q", lscpuOut, lscpuRe)
	}

	cmdLineOut := cmdOutput(cmdLineCommand)
	cmdLineMatchString := "intel_iommu=on"
	if !strings.Contains(cmdLineOut, cmdLineMatchString) {
		s.Fatalf("Failed to get cmdline info: got %q, want %q", cmdLineOut, cmdLineMatchString)
	}

	dmesgOut := cmdOutput(dmesgCommand)
	dmesgMatchString := "DMAR: IOMMU enabled"
	if !strings.Contains(dmesgOut, dmesgMatchString) {
		s.Fatalf("Failed to get dmesg DMAR info: got %q, want %q", dmesgOut, dmesgMatchString)
	}

	terminalWindowName := "Terminal - testuser@penguin: ~"
	if _, err = ash.BringWindowToForeground(ctx, tconn, terminalWindowName); err != nil {
		s.Fatal("Failed to bring the Terminal app to the front: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard event writer: ", err)
	}

	cmd := "sudo apt-get install cpu-checker --yes"
	if err := terminalApp.RunCommand(kb, cmd)(ctx); err != nil {
		s.Fatalf("Failed to run terminal app SSH %q command: %v", cmd, err)
	}

	cui := uiauto.New(tconn)
	installSuccessElement := nodewith.NameContaining("Processing triggers for man-db").Role(role.StaticText)
	if err := cui.WithTimeout(80 * time.Second).WaitUntilExists(installSuccessElement)(ctx); err != nil {
		s.Fatal("Failed to wait with timeout for cpu-checker install confirm text: ", err)
	}

	cmd = "sudo kvm-ok"
	if err := terminalApp.RunCommand(kb, cmd)(ctx); err != nil {
		s.Fatalf("Failed to run terminal app SSH %q command: %v", cmd, err)
	}

	kvmExistsElement := nodewith.Name("INFO: /dev/kvm exists").Role(role.StaticText)
	if err := cui.WaitUntilExists(kvmExistsElement)(ctx); err != nil {
		s.Fatal("Failed to wait for kvm exists UI text: ", err)
	}
	kvmUsedElement := nodewith.Name("KVM acceleration can be used").Role(role.StaticText)
	if err := cui.WaitUntilExists(kvmUsedElement)(ctx); err != nil {
		s.Fatal("Failed to wait for KVM acceleration used UI text: ", err)
	}
}
