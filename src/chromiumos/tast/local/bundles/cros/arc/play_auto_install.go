// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayAutoInstall,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "A functional test that verifies PlayAutoInstall(PAI) flow, It waits PAI is triggered and verifies the minimal set of apps is schedulled for installation",
		Contacts: []string{
			"arc-core@google.com",
			"khmel@chromium.org", // author.
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"arc_android_data_cros_access", "chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Val:               browser.TypeLacros,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros_vm",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Val:               browser.TypeLacros,
		}},
		Timeout: 4 * time.Minute,
		VarDeps: []string{"arc.PlayAutoInstall.username", "arc.PlayAutoInstall.password"},
	})
}

func PlayAutoInstall(ctx context.Context, s *testing.State) {
	// Note, ARC produces pailist.txt only for this account. Changing this account would lead to test failures.
	// TODO(khmel): Switch to pool of accounts "ui.gaiaPoolDefault".
	username := s.RequiredVar("arc.PlayAutoInstall.username")
	password := s.RequiredVar("arc.PlayAutoInstall.password")

	const (
		// Path to file to read of list of apps triggered by PlayAutoInstall flow (PAI).
		paiList = "/data/data/org.chromium.arc.gms/pailist.txt"
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	opts := []chrome.Option{
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.ARCSupported(),
		chrome.ExtraArgs("--arc-disable-app-sync", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"),
	}

	bt := s.Param().(browser.Type)
	cr, err := browserfixt.NewChrome(ctx, bt, lacrosfixt.NewConfig(), opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	s.Log("Performing optin")
	maxAttempts := 2
	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin: ", err)
	}

	// /data/data is not accessible from adb in RVC. Access this using chrome root.
	androidDataDir, err := arc.AndroidDataDir(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get android-data path: ", err)
	}

	paiListUnderHome := filepath.Join(androidDataDir, paiList)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	s.Log("Waiting PAI triggered")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// On ARCVM virtio-blk /data enabled devices, we mount and unmount the disk image on
		// every iteration of testing.Poll to ensure that the Android-side changes are
		// reflected on the host side.
		cleanupFunc, err := arc.MountVirtioBlkDataDiskImageReadOnlyIfUsed(ctx, a, cr.NormalizedUser())
		if err != nil {
			s.Fatal("Failed to make Android /data directory available on host: ", err)
		}
		defer cleanupFunc(cleanupCtx)

		if _, err := os.Stat(paiListUnderHome); err != nil {
			if os.IsNotExist(err) {
				return errors.Errorf("paiList %q is not created yet", paiListUnderHome)
			}
			return testing.PollBreak(err)
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute}); err != nil {
		s.Fatal("Failed to wait PAI triggered: ", err)
	}

	cleanupFunc, err := arc.MountVirtioBlkDataDiskImageReadOnlyIfUsed(ctx, a, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to make Android /data directory available on host: ", err)
	}
	defer cleanupFunc(cleanupCtx)

	data, err := ioutil.ReadFile(paiListUnderHome)
	if err != nil {
		s.Fatal("Failed to read PAI list: ", err)
	}

	paiDocs := make(map[string]bool)
	for _, doc := range strings.Split(string(data), "\n") {
		// Mark that app was not recognized as default at this momemnt.
		// List of know default apps will be applied to this map, and value
		// for each entry would be set to true. All other apps would be
		// considered as non-default app.
		if doc != "" {
			paiDocs[doc] = false
		}
	}

	if len(paiDocs) == 0 {
		// Common case that usually means PAI configuration is missing at server.
		s.Fatal("PAI was triggered but returned no app. Server configuration might be missed")
	}

	// Define default PAI list. Some boards might have extended set, however following must
	// exist on any board.
	defaultPaiDocs := []string{
		"com.google.android.deskclock",
		"com.google.android.apps.books",
		"com.google.android.play.games",
		"com.google.android.videos",
		"com.google.android.apps.youtube.music.pwa",
		"com.google.android.apps.photos"}
	// Verify that all default apps from the minimal set are scheduled for installation.
	for _, defaultDoc := range defaultPaiDocs {
		if _, ok := paiDocs[defaultDoc]; ok {
			s.Logf("Default app %q is found in the list", defaultDoc)
			paiDocs[defaultDoc] = true
		} else {
			s.Errorf("Default app %q was not found in the list. Server configuration might be outdated", defaultDoc)
		}
	}

	// Print leftover portion as board extra customization.
	for doc, found := range paiDocs {
		if !found {
			s.Logf("Found app %q outside of default list", doc)

		}
	}
}
