// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

const (
	configNamespace          = "arc_app_compat"
	o4cRecentAddedFlagName   = "o4c_recent_added"
	o4cRecentRemovedFlagName = "o4c_recent_removed"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeLockA2C2,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ARC++ Resize Lock via A2C2 works as expected",
		Contacts:     []string{"toshikikikuchi@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "arcBootedInClamshellModeWithArcUpdateO4CListViaA2C2",
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      5 * time.Minute,
	})
}

func ensureOverridePhenotypeFlag(ctx context.Context, a *arc.ARC) error {
	if err := a.Command(ctx, "device_config", "put", configNamespace, o4cRecentAddedFlagName, wm.ResizeLockO4CViaA2C2PkgName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to override phenotype flag for %s/%s", configNamespace, o4cRecentAddedFlagName)
	}
	if err := a.Command(ctx, "device_config", "put", configNamespace, o4cRecentRemovedFlagName, wm.ResizeLockNonO4CViaA2C2PkgName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to override phenotype flag for %s/%s", configNamespace, o4cRecentRemovedFlagName)
	}
	return nil
}

func testResizeLockState(ctx, cleanupCtx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, isNonO4CViaA2C2 bool) error {
	apkName := wm.ResizeLockO4CViaA2C2ApkName
	pkgName := wm.ResizeLockO4CViaA2C2PkgName
	if isNonO4CViaA2C2 {
		apkName = wm.ResizeLockNonO4CViaA2C2ApkName
		pkgName = wm.ResizeLockNonO4CViaA2C2PkgName
	}
	activityName := wm.ResizeLockMainActivityName

	// Install the test app.
	if err := a.Install(ctx, arc.APKPath(apkName), adb.InstallOptionFromPlayStore); err != nil {
		return errors.Wrap(err, "failed to install app from PlayStore")
	}
	defer a.Uninstall(cleanupCtx, pkgName)

	// GMS may periodically sync the flags so we need to override them everytime before we start the activity.
	if err := ensureOverridePhenotypeFlag(ctx, a); err != nil {
		return errors.Wrap(err, "failed to ensure to override the phenotype flag")
	}

	// Launch the test app.
	activity, err := arc.NewActivity(a, pkgName, activityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", activityName)
	}
	defer activity.Close()
	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start %s", activityName)
	}
	defer activity.Stop(cleanupCtx, tconn)
	if err := ash.WaitForVisible(ctx, tconn, activity.PackageName()); err != nil {
		return errors.Wrap(err, "failed to wait until the activity gets visible")
	}

	if isNonO4CViaA2C2 {
		// Close the compat mode splash dialog.
		if err := wm.CheckVisibility(ctx, tconn, wm.BubbleDialogClassName, true); err != nil {
			return errors.Wrap(err, "failed to wait for splash")
		}
		if err := wm.CloseSplash(ctx, tconn, wm.InputMethodClick, nil); err != nil {
			return errors.Wrap(err, "failed to close splash")
		}
	}

	// Verify the resize lock state.
	expectedMode := wm.NoneResizeLockMode
	if isNonO4CViaA2C2 {
		expectedMode = wm.PhoneResizeLockMode
	}
	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, expectedMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify the resize lock state of %s/%s", pkgName, activityName)
	}

	return nil
}

func ResizeLockA2C2(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice
	ui := uiauto.New(tconn).WithTimeout(5 * time.Second)

	dispInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get primary display info: ", err)
	}

	origShelfAlignment, err := ash.GetShelfAlignment(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf alignment: ", err)
	}
	if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentBottom); err != nil {
		s.Fatal("Failed to set shelf alignment to Bottom: ", err)
	}
	// Be nice and restore shelf alignment to its original state on exit.
	defer ash.SetShelfAlignment(cleanupCtx, tconn, dispInfo.ID, origShelfAlignment)

	origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Never Auto Hide: ", err)
	}
	// Be nice and restore shelf behavior to its original state on exit.
	defer ash.SetShelfBehavior(cleanupCtx, tconn, dispInfo.ID, origShelfBehavior)

	// Set a pure white wallpaper to reduce the noises on a screenshot because currently checking the visibility of the translucent window border relies on a screenshot.
	// The wallpaper will exist continuous if the Chrome session gets reused.
	if err := wm.SetSolidWhiteWallpaper(ctx, ui); err != nil {
		s.Fatal("Failed to set the white wallpaper: ", err)
	}

	if err := testResizeLockState(ctx, cleanupCtx, tconn, a, d, cr, true /* isNonO4CViaA2C2 */); err != nil {
		s.Fatal("Failed to verify the resize lock state of the app declared as non-O4C via A2C2: ", err)
	}
	if err := testResizeLockState(ctx, cleanupCtx, tconn, a, d, cr, false /* isNonO4CViaA2C2 */); err != nil {
		s.Fatal("Failed to verify the resize lock state of the app declared as O4C via A2C2: ", err)
	}
}
