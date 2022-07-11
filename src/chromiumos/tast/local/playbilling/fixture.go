// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/webapk"
	"chromiumos/tast/testing"
)

const (
	accountPool   = "ui.gaiaPoolDefault"
	assetLinksVar = "arc.PlayBillingAssetLinks"
	icon          = "play_billing_icon.png"
	index         = "play_billing_index.html"
	manifest      = "play_billing_manifest.json"
	payments      = "play_billing_payments.js"
	service       = "play_billing_service.js"
)

// pwaFiles are data files required to serve the Play Billing PWA.
var pwaFiles = []string{
	icon,
	index,
	manifest,
	payments,
	service,
}

// playBillingWebApk app helps test play billing.
var playBillingWebApk = webapk.WebAPK{
	Name:              "Play Billing Test PWA",
	ID:                "obcppbejhdfcplncjdlmagmpfjhmipii",
	Port:              8080,
	ApkDataPath:       "ArcPlayBillingTestPWA_20220210.apk",
	IndexPageDataPath: index,
}

// DataFiles are the files required for each Play Billing tests.
var DataFiles = append(pwaFiles, playBillingWebApk.ApkDataPath)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "arcBootedForPlayBilling",
		Desc:     "The fixture starts chrome with ARC supported used for Play Billing tests and disables popup blocking",
		Contacts: []string{"benreich@chromium.org", "jshikaram@chromium.org"},
		Impl: arc.NewArcBootedWithPlayStoreFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.ExtraArgs(append([]string{"--disable-popup-blocking"}, arc.DisableSyncFlags()...)...),
				chrome.GAIALoginPool(s.RequiredVar(accountPool))}, nil
		}),
		// Add two minutes to setup time to allow extra Play Store UI operations.
		SetUpTimeout: chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout: chrome.ResetTimeout,
		// Provide a longer enough PostTestTimeout value to fixture when ARC will try to dump ARCVM message.
		// Or there might be error of "context deadline exceeded".
		PostTestTimeout: arc.PostTestTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{accountPool},
	})

	testing.AddFixture(&testing.Fixture{
		Name:         "playBillingFixture",
		Desc:         "The fixture builds on arcBootedForPlayBilling but ensures the Play Billing PWA is started and the APK is sideloaded",
		Impl:         &playBillingFixture{},
		Contacts:     []string{"benreich@chromium", "jshikaram@chromium.org"},
		Parent:       "arcBootedForPlayBilling",
		Vars:         []string{assetLinksVar},
		Data:         DataFiles,
		SetUpTimeout: 2 * time.Minute,
	})
}

type playBillingFixture struct {
	wm     *webapk.Manager
	pwaDir string
}

// The FixtData object is made available to users of this fixture via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValue().(playbilling.FixtData)
//		...
//	}
type FixtData struct {
	TestApp *TestApp
}

func (f *playBillingFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	arcDevice := s.ParentValue().(*arc.PreData).ARC
	cr := s.ParentValue().(*arc.PreData).Chrome
	uiDevice := s.ParentValue().(*arc.PreData).UIDevice

	wm, err := webapk.NewManager(ctx, cr, cr.Browser(), arcDevice, s, playBillingWebApk)
	if err != nil {
		s.Fatal("Failed to create WebAPK Manager: ", err)
	}
	f.wm = wm

	// Install the test APK.
	if err := wm.InstallApk(ctx); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	pwaDir, err := ioutil.TempDir("", "tast-play-billing-pwa")
	if err != nil {
		s.Fatal("Failed to create temporary directory for Play Billing PWA: ", err)
	}
	f.pwaDir = pwaDir
	for _, name := range pwaFiles {
		pwaFilePath := filepath.Join(f.pwaDir, strings.TrimPrefix(name, "play_billing_"))
		if err := fsutil.CopyFile(s.DataPath(name), pwaFilePath); err != nil {
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

	wm.StartServerFromDir(ctx, f.pwaDir, func(err error) {
		s.Fatal("Failed to start PWA server: ", err)
	})

	testApp := NewTestApp(ctx, arcDevice, uiDevice, wm)

	return &FixtData{testApp}
}

func (f *playBillingFixture) Reset(ctx context.Context) error {
	// TODO: Ensure all PWA windows and ARC payment overlays are properly closed.
	return nil
}

func (f *playBillingFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *playBillingFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *playBillingFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if err := f.wm.ShutdownServer(ctx); err != nil {
		s.Log("Failed to shutdown PWA server: ", err)
	}

	if err := os.RemoveAll(f.pwaDir); err != nil {
		s.Logf("Failed to remove PWA directory %q: %v", f.pwaDir, err)
	}
}
