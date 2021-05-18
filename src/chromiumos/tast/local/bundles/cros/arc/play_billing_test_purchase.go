// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/playbilling"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PlayBillingTestPurchase,
		Desc: "Installs Play Billing test PWA and buys test SKU",
		Contacts: []string{
			"benreich@chromium.org",
			"jshikaram@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Timeout:      7 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "arcBootedForPlayBilling",
		Data: []string{
			"play_billing_icon.png",
			"play_billing_index.html",
			"play_billing_manifest.json",
			"play_billing_payments.js",
			"play_billing_service.js",
			"ArcPlayBillingTestPWA_20210517.apk",
		},
		Vars: []string{"arc.PlayBillingAssetLinks"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func PlayBillingTestPurchase(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	arcDevice := s.FixtValue().(*arc.PreData).ARC

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := arcDevice.Install(ctx, s.DataPath("ArcPlayBillingTestPWA_20210517.apk")); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	assetLinks, _ := s.Var("arc.PlayBillingAssetLinks")

	pwaDir, err := ioutil.TempDir("", "tast.filemanager.PlayBillingTestPWA.")
	if err != nil {
		s.Fatal("Failed creating temp PWA directory: ", err)
	}
	defer os.RemoveAll(pwaDir)

	for _, name := range []string{"manifest.json", "icon.png", "index.html", "payments.js", "service.js"} {
		if err := fsutil.CopyFile(s.DataPath("play_billing_"+name), filepath.Join(pwaDir, name)); err != nil {
			s.Fatalf("Failed copying %q to temp directory %q: %v", "play_billing_"+name, pwaDir, err)
		}
	}

	if err := os.MkdirAll(filepath.Join(pwaDir, ".well-known"), 0700); err != nil {
		s.Fatal("Failed to create the .well-known directory: ", err)
	}

	testFileLocation := filepath.Join(pwaDir, ".well-known", "assetlinks.json")
	if err := ioutil.WriteFile(testFileLocation, []byte(assetLinks), 0644); err != nil {
		s.Fatalf("Failed creating %q: %s", testFileLocation, err)
	}

	pbPwa, err := playbilling.NewTestPWA(ctx, cr, arcDevice, pwaDir)
	if err != nil {
		s.Fatal("Failed setting up test PWA: ", err)
	}
	defer pbPwa.Close(cleanupCtx)

	if err := pbPwa.BuySKU(ctx, "android.test.purchased"); err != nil {
		s.Fatal("Failed to click on the test SKU button: ", err)
	}

	// TODO(jshikaram): Implement verification and interaction with the ARC Payments overlay.
}
