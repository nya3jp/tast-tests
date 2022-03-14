// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/arc/removablemedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnableExternalStorage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies ARC removable media can be enabled from Chrome OS Settings",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com", "arc-storage@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 1*time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func EnableExternalStorage(ctx context.Context, s *testing.State) {
	// Set up Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Optin to PlayStore and close the app.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	const (
		imageSize   = 64 * 1024 * 1024
		diskName    = "MyDisk"
		androidPath = "/media/removable/MyDisk/Android"
	)

	// Set up a filesystem image.
	image, err := removablemedia.CreateZeroFile(imageSize, "vfat.img")
	if err != nil {
		s.Fatal("Failed to create image: ", err)
	}
	defer os.Remove(image)

	devLoop, err := removablemedia.AttachLoopDevice(ctx, image)
	if err != nil {
		s.Fatal("Failed to attach loop device: ", err)
	}
	defer func() {
		if err := removablemedia.DetachLoopDevice(ctx, devLoop); err != nil {
			s.Error("Failed to detach from loop device: ", err)
		}
	}()

	if err := removablemedia.FormatVFAT(ctx, devLoop); err != nil {
		s.Fatal("Failed to format VFAT file system: ", err)
	}

	// Mount the image via CrosDisks.
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to find crosdisks D-Bus service: ", err)
	}
	_, err = removablemedia.Mount(ctx, cd, devLoop, diskName)
	if err != nil {
		s.Fatal("Failed to mount file system: ", err)
	}
	defer func() {
		if err := removablemedia.Unmount(ctx, cd, devLoop); err != nil {
			s.Error("Failed to unmount VFAT image: ", err)
		}
	}()

	if err := arc.WaitForARCRemovableMediaVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for the volume to be mounted in ARC: ", err)
	}

	const (
		// This is a plain app that triggers "Android" folder creation when external storage permission is ON.
		apk = "ArcExternalStorageTest.apk"
		pkg = "org.chromium.arc.testapp.externalstoragetast"
		cls = ".MainActivity"
	)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting app")
	if err = act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	if err = act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	// Verify Android dir is not present.
	_, err = ioutil.ReadDir(androidPath)
	if os.IsNotExist(err) {
		s.Log("Android folder doesn't exist: ", err)
	}
	if err == nil {
		s.Fatal("Android exists: ", err)
	}

	// Enable External Storage Permission from Chrome OS Settings.
	ui := uiauto.New(tconn)
	externalStoragePreferenceButton := nodewith.Name("External storage preferences").Role(role.Link)
	myDiskButton := nodewith.Name(diskName).Role(role.ToggleButton)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "storage", ui.Exists(externalStoragePreferenceButton)); err != nil {
		s.Fatal("Failed to launch apps settings page: ", err)
	}

	if err := uiauto.Combine("Toggle External Storage Settings",
		ui.FocusAndWait(externalStoragePreferenceButton),
		ui.LeftClick(externalStoragePreferenceButton),
		ui.LeftClick(myDiskButton),
	)(ctx); err != nil {
		s.Fatal("Failed to Open Storage Settings : ", err)
	}

	if err := arc.WaitForARCRemovableMediaVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for the volume to be mounted in ARC: ", err)
	}

	s.Log("Restarting app")
	if err = act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	// Verify Android dir is present.
	_, err = ioutil.ReadDir(androidPath)
	if os.IsNotExist(err) {
		s.Fatal("Android folder doesn't exist: ", err)
	}
	if err != nil {
		s.Fatal("Failed to read Android folder: ", err)
	}
}
