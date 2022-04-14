// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HermesESIMUpdate,
		Desc: "Tests the ESIM OS update flow",
		Contacts: []string{
			"pholla@google.com",
			"chromeos-cellular-team@google.com",
		},
		Data: []string{
			"third_party/Test_multiscript_chrome_v1.xml",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_prod_esim"},
		Timeout: 3 * time.Minute,
	})
}

func HermesESIMUpdate(ctx context.Context, s *testing.State) {
	f, err := os.CreateTemp("", "invalid-update.xml")
	if err != nil {
		s.Fatal("Could not create invalid EOS FW xml: ", err)
	}
	defer os.Remove(f.Name())

	if err := performFWUpdate(ctx, f.Name(), false); err != nil {
		s.Fatal("Could not test EOS update with invalid file: ", err)
	}
	if err := performFWUpdate(ctx, s.DataPath("third_party/Test_multiscript_chrome_v1.xml"), true); err != nil {
		s.Fatal("EOS update failed: ", err)
	}
}

func performFWUpdate(ctx context.Context, fwPath string, isValidUpdate bool) error {
	if err := os.Chmod(fwPath, 0644); err != nil {
		return errors.Wrapf(err, "unable to change permissions of %q", fwPath)
	}
	const hermesJobName = "hermes"
	if err := upstart.RestartJob(ctx, hermesJobName, upstart.WithArg("LOG_LEVEL", "-2"), upstart.WithArg("ESIM_FW_PATH", fwPath)); err != nil {
		return errors.Wrapf(err, "failed to restart %q", hermesJobName)
	}
	euicc, _, err := hermes.WaitForEUICC(ctx, false)
	if err != nil {
		return errors.Wrap(err, "unable to get hermes euicc")
	}
	// The following Hermes call triggers an eOS update
	_, err = euicc.InstalledProfiles(ctx, true)
	if !isValidUpdate && err == nil {
		return errors.New("EOS update should fail for invalid FW")
	}
	if isValidUpdate && err != nil {
		return errors.Wrap(err, "failed to get installed profiles after EOS update")
	}
	return nil
}
