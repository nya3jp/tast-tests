// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	// Timeout for the test and for installing Android packages is 4 minutes.
	timeout = 4 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcProvisioning,
		Desc:         "Checks that ARC is launched when policy is set",
		Contacts:     []string{"pbond@chromium.org", "arc-eng-muc@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"credentials.json"},
		Timeout:      timeout,
	})
}

// Credentials must be kept in sync with credentials.json file.
type credentials struct {
	User                   string   `json:"user"`
	Password               string   `json:"password"`
	GaiaID                 string   `json:"gaiaId"`
	ForceInstalledPackages []string `json:forceInstalledPackages"`
}

// ArcProvisioning runs the provisioning smoke test:
// - login with managed account,
// - check that ARC is launched by user policy,
// - check that force-installed by policy Android packages are installed.
func ArcProvisioning(ctx context.Context, s *testing.State) {
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
	for pkg := range creds.ForceInstalledPackages {
		if err := waitForPackage(ctx, a, creds.ForceInstalledPackages[pkg]); err != nil {
			s.Fatal("Failed to force install packages: ", err)
		}
	}
}

// getCreds reads credentials from files stored in private repository.
func getCreds(path string) (*credentials, error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read json file with credentials")
	}
	var creds credentials
	if err := json.Unmarshal([]byte(f), &creds); err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling credentials")
	}
	return &creds, nil
}

// waitForPackage waits for Android package being installed. Max 4 minutes.
func waitForPackage(ctx context.Context, a *arc.ARC, packageName string) error {
	ctx, st := timing.Start(ctx, fmt.Sprintf("wait_package_%s", packageName))
	defer st.End()

	testing.ContextLog(ctx, "Waiting for package being installed ", packageName)

	return testing.Poll(ctx, func(ctx context.Context) error {
		cmd := a.Command(ctx, "pm", "list", "packages", "-3")
		out, err := cmd.Output()
		if err != nil {
			cmd.DumpLog(ctx)
			return errors.Wrap(err, "pm list -3 failed")
		}

		pkgs := strings.Split(string(out), "\n")
		found := false
		for _, p := range pkgs {
			if p == fmt.Sprintf("package:%s", packageName) {
				found = true
				break
			}
		}
		if !found {
			return errors.New("package is not installed yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}
