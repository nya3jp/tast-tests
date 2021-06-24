// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/arc/removablemedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnableExternalStorage,
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
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 4*time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func EnableExternalStorage(ctx context.Context, s *testing.State) {
	// Setup Chrome.
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

	// Optin to PlayStore and Close
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	const (
		imageSize = 64 * 1024 * 1024
		diskName  = "MyDisk"
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

	if err := removablemedia.WaitForARCVolumeMount(ctx, a); err != nil {
		s.Fatal("Failed to wait for the volume to be mounted in ARC: ", err)
	}

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := filesApp.OpenDir("MyDisk", "Files - MyDisk")(ctx); err != nil {
			return errors.Wrap(err, "failed to open MyDisk folder")
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Second, Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed to find MyDisk folder: ", err)
	}

	// Verify that Android folder is not present.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := filesApp.WaitForFile("Android")(ctx); err == nil {
			return errors.Wrap(err, "Android folder present")
		}
		return errors.Wrap(err, "Android folder not present")
	}, &testing.PollOptions{Interval: 10 * time.Second, Timeout: 30 * time.Second}); err != nil {
		s.Log("Verified Android folder is not present: ", err)
	}

	// Enable External Storage Permission from Chrome OS Settings.
	ui := uiauto.New(tconn)
	externalStoragePreferenceButton := nodewith.Name("External storage preferences").Role(role.Link)
	myDiskButton := nodewith.Name("MyDisk").Role(role.ToggleButton)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "storage", ui.Exists(externalStoragePreferenceButton)); err != nil {
		s.Fatal("Failed to launch apps settings page: ", err)
	}

	if err := uiauto.Combine("Open Android Settings",
		ui.FocusAndWait(externalStoragePreferenceButton),
		ui.LeftClick(externalStoragePreferenceButton),
		ui.LeftClick(myDiskButton),
	)(ctx); err != nil {
		s.Fatal("Failed to Open Android Settings : ", err)
	}

	const (
		// This is a plain app calling getExternalFilesDir(null) so that "Android" folder is created.
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
	if err = act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	// Open Files app.
	filesApp, err = filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(ctx)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := filesApp.OpenDir("MyDisk", "Files - MyDisk")(ctx); err != nil {
			return errors.Wrap(err, "failed to open MyDisk folder")
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Second, Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed to find MyDisk folder: ", err)
	}

	// Verify the Android folder.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := filesApp.WaitForFile("Android")(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for Android folder")
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Second, Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to find Android folder: ", err)
	}
}
