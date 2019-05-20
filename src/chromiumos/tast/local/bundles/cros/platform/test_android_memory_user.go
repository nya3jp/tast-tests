// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/platform/memoryuser"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestAndroidMemoryUser,
		Desc:         "Tests heavy memory use with Chrome, ARC and VMs running",
		Contacts:     []string{"asavery@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      10 * time.Minute,
		Data:         []string{"memory_user_NOVALegacy-1.1.5.apk"},
		SoftwareDeps: []string{"android", "chrome_login", "vm_host"},
	})
}

func TestAndroidMemoryUser(ctx context.Context, s *testing.State) {
	const (
		// This starts the NOVA Legacy game
		apk            = "memory_user_NOVALegacy-1.1.5.apk"
		pkg            = "com.gameloft.android.ANMP.GloftNOHM"
		activityName   = ".MainActivity"
		className      = "android.view.View"
		screenshotPath = "/tmp/screenshot.png"
		pkgNameGMS     = "com.google.android.gms"
		timeoutWaitGMS = 20000
	)

	must := func(err error) {
		if err != nil {
			s.Fatal("Something Failed: ", err)
		}
	}

	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)

	getPixel := func(x int, y int) color.Color {
		cmd := testexec.CommandContext(ctx, "screenshot", fmt.Sprintf("--crop=1x1+%d+%d", x, y), screenshotPath)
		if err := cmd.Run(); err != nil {
			testing.ContextLog(ctx, "screenshot command failed: ", err)
		}
		imgFile, err := os.Open(screenshotPath)
		if err != nil {
			s.Fatalf("Could not open %s: ", screenshotPath, err)
		}
		defer imgFile.Close()
		img, _, err := image.Decode(imgFile)
		return img.At(0, 0)
	}

	arcNovaFunc := func(a *arc.ARC, d *ui.Device) {
		viewport := d.Object(ui.ClassName(className), ui.PackageName(pkg))
		must(viewport.WaitForExists(ctx, 60*time.Second))
		dInfo, err := d.GetInfo(ctx)
		if err != nil {
			s.Fatal("Failed to get UI device info: ", err)
		}
		x := int(0.5 * float64(dInfo.DisplayWidth))
		y := int(0.2 * float64(dInfo.DisplayHeight))
		stillBlackErr := errors.New("Pixel still black, resource extraction hasn't finished")
		err = testing.Poll(ctx, func(ctx context.Context) error {
			clr := getPixel(x, y)
			r, g, b, _ := clr.RGBA()
			if r != 0 || g != 0 || b != 0 {
				return nil
			}
			return stillBlackErr
		}, &testing.PollOptions{Timeout: 180 * time.Second})
		if err != nil {
			s.Fatal("Failed to extract resource: ", err)
		}

		testing.Sleep(ctx, 30*time.Second)
		x = int(0.5 * float64(dInfo.DisplayWidth))
		y = int(.28 * float64(dInfo.DisplayHeight))
		d.Click(ctx, x, y)
		_, err = a.Command(ctx, "input", "text", "21").Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatal("Failed to get display window info")
		}

		x = int(.45 * float64(dInfo.DisplayWidth))
		y = int(0.35 * float64(dInfo.DisplayHeight))
		d.Click(ctx, x, y)

		x = int(.5 * float64(dInfo.DisplayWidth))
		y = int(.855 * float64(dInfo.DisplayHeight))
		d.Click(ctx, x, y)

		testing.Sleep(ctx, 5*time.Second)

		x = int(.5 * float64(dInfo.DisplayWidth))
		y = int(.5 * float64(dInfo.DisplayHeight))
		d.Click(ctx, x, y)
		testing.Sleep(ctx, 1*time.Second)
		d.Click(ctx, x, y)
		testing.Sleep(ctx, 3*time.Second)
		startTime := time.Now()

		gmsCancel := d.Object(ui.Text("CANCEL"), ui.PackageName(pkgNameGMS))
		must(gmsCancel.WaitForExists(ctx, 30*time.Second))
		gmsCancel.Click(ctx)

		testing.Sleep(ctx, 20*time.Second)

		originalColor := getPixel(x, y)
		loadErr := errors.New("Pixel color unchanged, loading not complete")
		err = testing.Poll(ctx, func(ctx context.Context) error {
			clr := getPixel(x, y)
			if clr != originalColor {
				return nil
			}
			return loadErr
		}, &testing.PollOptions{Timeout: 90 * time.Second})
		if err != nil {
			s.Fatal("Failed to load level: ", err)
		}
		loadingTime := time.Now().Sub(startTime).Seconds()
		s.Logf("Loading time: %f seconds", loadingTime)
	}

	aTask := memoryuser.AndroidTask{APKPath: s.DataPath(apk), APK: apk, Pkg: pkg, ActivityName: activityName, TestFunc: arcNovaFunc}
	memTasks := []memoryuser.MemoryTask{&aTask}

	memoryuser.RunTest(ctx, s, memTasks)
}
