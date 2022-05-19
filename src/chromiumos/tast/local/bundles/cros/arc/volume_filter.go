// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/arc/removablemedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VolumeFilter,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks ArcDocumentsUI's OPEN_MEDIA_STORE_FILES intent action works as expected",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-storage@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Data:         []string{"capybara.jpg"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func VolumeFilter(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(
		ctx,
		chrome.ARCSupported(),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ExtraArgs(arc.DisableSyncFlags()...),
	)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := optin.PerformWithRetry(ctx, cr, 1); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	if err := arc.WaitForARCMyFilesVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for MyFiles to be mounted in ARC: ", err)
	}

	const (
		imageSize = 64 * 1024 * 1024
		diskName  = "MyDisk"
	)
	if _, _, err := removablemedia.CreateAndMountImage(ctx, imageSize, diskName); err != nil {
		s.Fatal("Failed to set up removable media: ", err)
	}

	if err := arc.WaitForARCRemovableMediaVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for removable media to be mounted in ARC: ", err)
	}

	capybara, err := ioutil.ReadFile(s.DataPath("capybara.jpg"))
	if err != nil {
		s.Fatal("Failed to read test file: ", err)
	}

	cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get cryptohome user path for %s: %v", cr.NormalizedUser(), err)
	}

	androidDataDir, err := arc.AndroidDataDir(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get android-data directory for %s: %v", cr.NormalizedUser(), err)
	}

	filenames := []string{"capybara.jpg", "capybara (1).jpg"}
	for _, filename := range filenames {
		paths := []string{
			filepath.Join("/media/removable", diskName, filename),
			filepath.Join(cryptohomeUserPath, "MyFiles", filename),
			filepath.Join(cryptohomeUserPath, "MyFiles", "Downloads", filename),

			filepath.Join(androidDataDir, "data", "media", "0", "Pictures", filename),
		}
		for _, path := range paths {
			if err = ioutil.WriteFile(path, capybara, 0666); err != nil {
				s.Fatalf("Could not write to %s: %v", path, err)
			}
		}
	}

	const (
		apk = "ArcVolumeFilterTest.apk"
		pkg = "org.chromium.arc.testapp.volumefilter"
		cls = "org.chromium.arc.testapp.volumefilter.MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk), adb.InstallOptionGrantPermissions); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	s.Log("Starting app")
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start activity: ", err)
	}
	act.Close()
}
