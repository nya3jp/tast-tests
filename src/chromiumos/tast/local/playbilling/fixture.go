// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	userVar         = "arc.PlayBillingUser"
	passVar         = "arc.PlayBillingPass"
	assetLinksVar   = "arc.PlayBillingAssetLinks"
	apk             = "ArcPlayBillingTestPWA_20210517.apk"
	localServerPort = 80
)

// pwaFiles are data files required to serve the Play Billing PWA.
// TODO(b/187793648): Move this to the fixture once data is supported.
var pwaFiles = []string{
	"play_billing_icon.png",
	"play_billing_index.html",
	"play_billing_manifest.json",
	"play_billing_payments.js",
	"play_billing_service.js",
}

// DataFiles are the files required for each Play Billing tests.
// TODO(b/187793648): Move this to the fixture once data is supported.
var DataFiles = append(pwaFiles, apk)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "arcBootedForPlayBilling",
		Desc:     "The fixture starts chrome with ARC supported used for Play Billing tests",
		Contacts: []string{"benreich@chromium.org", "jshikaram@chromium.org"},
		Impl: arc.NewArcBootedWithPlayStoreFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			userID, ok := s.Var(userVar)
			if !ok {
				s.Fatalf("Runtime variable %s is not provided", userVar)
			}
			userPasswd, ok := s.Var(passVar)
			if !ok {
				s.Fatalf("Runtime variable %s is not provided", passVar)
			}

			return []chrome.Option{
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
				chrome.ARCSupported(),
				chrome.GAIALogin(chrome.Creds{User: userID, Pass: userPasswd})}, nil
		}),
		// Add two minutes to setup time to allow extra Play Store UI operations.
		SetUpTimeout: chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout: chrome.ResetTimeout,
		// Provide a longer enough PostTestTimeout value to fixture when ARC will try to dump ARCVM message.
		// Or there might be error of "context deadline exceeded".
		PostTestTimeout: 5 * time.Second,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{userVar, passVar},
	})

	testing.AddFixture(&testing.Fixture{
		Name:         "playBillingFixture",
		Desc:         "The fixture builds on arcBootedForPlayBilling but ensures the Play Billing PWA is started and the APK is sideloaded",
		Impl:         &playBillingFixture{},
		Contacts:     []string{"benreich@chromium", "jshikaram@chromium.org"},
		Parent:       "arcBootedForPlayBilling",
		Vars:         []string{assetLinksVar},
		SetUpTimeout: 2 * time.Minute,
	})
}

type playBillingFixture struct {
	pwaServer *http.Server
	pwaDir    string
}

// The FixtData object is made available to users of this fixture via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValue().(playbilling.FixtData)
//		...
//	}
type FixtData struct {
	ARC     *arc.ARC
	Chrome  *chrome.Chrome
	TestApp *TestApp
}

func (f *playBillingFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	arcDevice := s.ParentValue().(*arc.PreData).ARC
	cr := s.ParentValue().(*arc.PreData).Chrome

	// TODO(b/187793648): Move this to the fixture once data is supported.
	const prebuiltLocalDataPath = "/usr/local/share/tast/data/chromiumos/tast/local/bundles/cros/appsplatform/data"
	const builtLocalDataPath = "/usr/local/share/tast/data_pushed/chromiumos/tast/local/bundles/cros/appsplatform/data"

	// TODO(b/187793648): Replace with s.DataPath(apk) when data is supported in Fixtures.
	// The data path changes based on whether -build=true or -build=false is supplied to `tast run`.
	// Local test runs on your workstation use -build=true by default, while lab runs use -build=false.

	// Use the built local data path if it exists, and fall back to the prebuilt data path otherwise.
	localDataPath := builtLocalDataPath
	if _, err := os.Stat(builtLocalDataPath); os.IsNotExist(err) {
		localDataPath = prebuiltLocalDataPath
	} else if err != nil {
		s.Fatal("Failed to check if built local data path exists: ", err)
	}

	// Install the test APK.
	if err := arcDevice.Install(ctx, filepath.Join(localDataPath, apk)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	pwaDir, err := ioutil.TempDir("", "tast-play-billing-pwa")
	if err != nil {
		s.Fatal("Failed to create temporary directory for Play Billing PWA: ", err)
	}
	f.pwaDir = pwaDir
	for _, name := range pwaFiles {
		pwaFilePath := filepath.Join(f.pwaDir, strings.TrimPrefix(name, "play_billing_"))
		if err := fsutil.CopyFile(filepath.Join(localDataPath, name), pwaFilePath); err != nil {
			s.Fatalf("Failed to copy extension file %q: %v", name, err)
		}
	}

	assetLinks, ok := s.Var(assetLinksVar)
	if !ok {
		s.Fatal("Failed retrieving runtime variable: ", assetLinksVar)
	}

	wellKnownDirectory := filepath.Join(f.pwaDir, ".well-known")
	if err := os.MkdirAll(wellKnownDirectory, 0700); err != nil {
		s.Fatal("Failed to create the .well-known directory: ", err)
	}

	testFileLocation := filepath.Join(wellKnownDirectory, "assetlinks.json")
	if err := ioutil.WriteFile(testFileLocation, []byte(assetLinks), 0644); err != nil {
		s.Fatalf("Failed creating %q: %s", testFileLocation, err)
	}

	fs := http.FileServer(http.Dir(f.pwaDir))
	f.pwaServer = &http.Server{Addr: fmt.Sprintf(":%v", localServerPort), Handler: fs}
	go func() {
		if err := f.pwaServer.ListenAndServe(); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "Failed to create local server: ", err)
		}
	}()

	testApp, err := NewTestApp(ctx, cr, arcDevice)
	if err != nil {
		s.Fatal("Failed trying to setup test app: ", err)
	}

	return &FixtData{arcDevice, cr, testApp}
}

func (f *playBillingFixture) Reset(ctx context.Context) error {
	// TODO: Ensure all PWA windows and ARC payment overlays are properly closed.
	return nil
}

func (f *playBillingFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *playBillingFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *playBillingFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.pwaServer.Shutdown(ctx); err != nil {
		s.Log("Failed to shutdown PWA server: ", err)
	}

	if err := os.RemoveAll(f.pwaDir); err != nil {
		s.Logf("Failed to remove PWA directory %q: %v", f.pwaDir, err)
	}
}
