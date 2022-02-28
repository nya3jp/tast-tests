// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayAutoInstall,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "A functional test that verifies PlayAutoInstall(PAI) flow, It waits PAI is triggered and verifies the minimal set of apps is schedulled for installation",
		Contacts: []string{
			"arc-core@google.com",
			"khmel@chromium.org", // author.
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 5 * time.Minute,
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

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.ARCSupported(),
		chrome.ExtraArgs("--arc-disable-app-sync", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	s.Log("Performing optin")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin: ", err)
	}

	// /data/data is not accessible from adb in RVC. Access this using chrome root.
	androidDataDir, err := arc.AndroidDataDir(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get android-data path: ", err)
	}

	paiListUnderHome := filepath.Join(androidDataDir, paiList)

	s.Log("Waiting PAI triggered")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
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
