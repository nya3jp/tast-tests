// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCProvisioning,
		Desc:         "Checks that ARC is launched when policy is set",
		Contacts:     []string{"pbond@chromium.org", "arc-eng-muc@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"credentials.json"},
		Timeout:      4 * time.Minute,
	})
}

// Credentials must be kept in sync with credentials.json file.
type credentials struct {
	User                   string   `json:"user"`
	Password               string   `json:"password"`
	GaiaID                 string   `json:"gaiaId"`
	ForceInstalledPackages []string `json:"forceInstalledPackages"`
}

// ARCProvisioning runs the provisioning smoke test:
// - login with managed account,
// - check that ARC is launched by user policy,
// - check that force-installed by policy Android packages are installed.
func ARCProvisioning(ctx context.Context, s *testing.State) {
	creds, err := getCreds(s.DataPath("credentials.json"))
	if err != nil {
		s.Fatal("Failed to read credentials: ", err)
	}
	// Log-in to Chrome and allow to launch ARC if allowed by user policy.
	cr, err := chrome.New(
		ctx,
		chrome.Auth(creds.User, creds.Password, creds.GaiaID),
		chrome.GAIALogin(),
		chrome.ExtraArgs("--arc-availability=officially-supported"),
		chrome.FetchPolicy())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Ensure that ARC is launched.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
	defer a.Close()

	// Ensure that Android packages are force-installed by ARC policy.
	// Note: if the user policy for the user is changed, the packages listed in
	// credentials.json must be updated.
	if err := waitForPackages(ctx, a, creds.ForceInstalledPackages); err != nil {
		s.Fatal("Failed to force install packages: ", err)
	}
}

// getCreds reads credentials from files stored in private repository.
func getCreds(path string) (*credentials, error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read json file with credentials")
	}
	var creds credentials
	if err := json.Unmarshal(f, &creds); err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling credentials")
	}
	return &creds, nil
}

// waitForPackages waits for Android package being installed.
func waitForPackages(ctx context.Context, a *arc.ARC, packages []string) error {
	ctx, st := timing.Start(ctx, "wait_packages")
	defer st.End()

	notInstalledPackages := make(map[string]bool)
	for _, p := range packages {
		notInstalledPackages[p] = true
	}

	testing.ContextLog(ctx, "Waiting for packages being installed")

	return testing.Poll(ctx, func(ctx context.Context) error {
		cmd := a.Command(ctx, "pm", "list", "packages", "-3")
		out, err := cmd.Output()
		if err != nil {
			cmd.DumpLog(ctx)
			return errors.Wrap(err, "pm list -3 failed")
		}

		pkgs := strings.Split(string(out), "\n")
		for _, p := range pkgs {
			if notInstalledPackages[strings.TrimPrefix(p, "package:")] {
				delete(notInstalledPackages, strings.TrimPrefix(p, "package:"))
			}
		}
		if len(notInstalledPackages) != 0 {
			return errors.Errorf("%d package(s) are not installed yet",
				len(notInstalledPackages))
		}
		return nil
	}, nil)
}
