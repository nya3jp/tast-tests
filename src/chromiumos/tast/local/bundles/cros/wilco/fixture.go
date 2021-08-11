// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wilco/wilcoextension"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	// "chromiumos/tast/local/vm"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "wilcoDTCEnrolled",
		Desc: "Wilco DTC enrollment fixture with Wilco DTC VM and Supportd daemon",
		Contacts: []string{
			"lamzin@google.com", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
			"bisakhmondal00@gmail.com",
		},
		Impl:            NewWilcoFixture(false),
		Parent:          "chromeEnrolledLoggedIn",
		SetUpTimeout:    60 * time.Second,
		PreTestTimeout:  20 * time.Second,
		PostTestTimeout: 10 * time.Second,
		TearDownTimeout: 15 * time.Second,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "wilcoDTCEnrolledExtensionSupport",
		Desc: "Wilco DTC enrollment fixture with Wilco Test Extension, Wilco DTC VM and Supportd daemon",
		Contacts: []string{
			"lamzin@google.com", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
			"bisakhmondal00@gmail.com",
		},
		Impl:            NewWilcoFixture(true),
		Parent:          "fakeDMSEnrolled",
		SetUpTimeout:    60 * time.Second,
		PreTestTimeout:  20 * time.Second,
		ResetTimeout:    chrome.ResetTimeout,
		PostTestTimeout: 10 * time.Second,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

// NewWilcoFixture returns an object that implements FixtureImpl interface.
func NewWilcoFixture(fdmsOnly bool) *wilcoEnrolledFixture {
	return &wilcoEnrolledFixture{
		onlyFakeDMS: fdmsOnly,
	}
}

// wilcoEnrolledFixture implements testing.FixtureImpl.
type wilcoEnrolledFixture struct {
	wilcoDTCVMPID       int
	wilcoDTCSupportdPID int
	// onlyFakeDMS states if the parent fixture is a fakeDMSEnrolled fixture.
	onlyFakeDMS bool
	// Local chrome & fdms state.
	cr   *chrome.Chrome
	fdms *fakedms.FakeDMS
	// Extension directory where asstes are stored temporarily.
	extensionDir string
}

func (w *wilcoEnrolledFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if w.onlyFakeDMS {
		fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
		if !ok {
			s.Fatal("Parent is not a FakeDMS fixture")
		}

		extDir, err := ioutil.TempDir("", "tast.ChromeExtension.")
		if err != nil {
			s.Fatal("Failed to create temp dir: ", err)
		}
		w.extensionDir = extDir

		s.Log("Writing unpacked extension to ", extDir)
		if err := ioutil.WriteFile(filepath.Join(extDir, "manifest.json"), []byte(wilcoextension.Manifest), 0644); err != nil {
			s.Fatal("Failed to write manifest.json: ", err)
		}
		if err := ioutil.WriteFile(filepath.Join(extDir, "background.js"), []byte{}, 0644); err != nil {
			s.Fatal("Failed to write background.js: ", err)
		}
		if extID, err := chrome.ComputeExtensionID(extDir); err != nil {
			s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
		} else if extID != wilcoextension.ID {
			s.Fatalf("Unexpected extension id: got %s; want %s", extID, wilcoextension.ID)
		}

		cr, err := chrome.New(ctx, chrome.KeepEnrollment(),
			chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
			chrome.DMSPolicy(fdms.URL),
			chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout),
			chrome.UnpackedExtension(extDir))
		if err != nil {
			s.Fatal("Chrome startup failed: ", err)
		}

		w.cr = cr
		w.fdms = fdms
	} else {
		w.cr = s.ParentValue().(*fixtures.FixtData).Chrome
		w.fdms = s.ParentValue().(*fixtures.FixtData).FakeDMS
	}

	// =======================================[Not Working] Enrollment is in SetUp() ==============
	// // Performing policy enrollment
	// pb := fakedms.NewPolicyBlob()
	// // wilco_dtc and wilco_dtc_supportd only run for affiliated users.
	// pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
	// pb.UserAffiliationIds = []string{"default_affiliation_id"}

	// // After this point, IsUserAffiliated flag should be updated.
	// if err := policyutil.ServeBlobAndRefresh(ctx, w.fdms, w.cr, pb); err != nil {
	// 	s.Fatal("Failed to serve and refresh: ", err)
	// }

	// // We should add policy value in the middle of 2 ServeBlobAndRefresh calls to be sure
	// // that IsUserAffiliated flag is updated and policy handler is triggered.
	// pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true})

	// // After this point, the policy handler should be triggered.
	// if err := policyutil.ServeBlobAndRefresh(ctx, w.fdms, w.cr, pb); err != nil {
	// 	s.Fatal("Failed to serve and refresh: ", err)
	// }

	// // Ensuring the VM is ready to run commands over vsh.
	// if err := testing.Poll(ctx, func(ctx context.Context) error {
	// 	return vm.CreateVSHCommand(ctx, wilco.WilcoVMCID, "true").Run()
	// }, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
	// 	s.Fatal("Failed to wait for DTC VM to be ready: ", err)
	// }

	// ============================================================================================
	// ===================================If enrollment is not in SetUp() =========================
	// Starting wilco DTC vm.
	if err := wilco.StartVM(ctx, &wilco.VMConfig{
		StartProcesses: false,
		TestDBusConfig: false,
	}); err != nil {
		s.Fatal("Failed to start the Wilco DTC VM: ", err)
	}
	// ============================================================================================
	pidVM, err := wilco.VMPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC VM PID: ", err)
	}
	w.wilcoDTCVMPID = pidVM

	return &fixtures.FixtData{
		FakeDMS: w.fdms,
		Chrome:  w.cr,
	}
}

func (w *wilcoEnrolledFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// ======================================== [Working] If enrollment is in PreTest() ===========
	// Performing policy enrollment
	pb := fakedms.NewPolicyBlob()
	// wilco_dtc and wilco_dtc_supportd only run for affiliated users.
	pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
	pb.UserAffiliationIds = []string{"default_affiliation_id"}

	// After this point, IsUserAffiliated flag should be updated.
	if err := policyutil.ServeBlobAndRefresh(ctx, w.fdms, w.cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	// We should add policy value in the middle of 2 ServeBlobAndRefresh calls to be sure
	// that IsUserAffiliated flag is updated and policy handler is triggered.
	pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true})

	// After this point, the policy handler should be triggered.
	if err := policyutil.ServeBlobAndRefresh(ctx, w.fdms, w.cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}
	// ============================================================================================

	// Restart the Wilco DTC Daemon to flush the queued events.
	if err := wilco.StartSupportd(ctx); err != nil {
		s.Fatal("Failed to restart the Wilco DTC Support Daemon: ", err)
	}

	pid, err := wilco.SupportdPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC Support Daemon PID: ", err)
	}

	w.wilcoDTCSupportdPID = pid
}

func (w *wilcoEnrolledFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Ensures that test doesn't interfere with the Wilco DTC VM and Daemon.
	pid, err := wilco.VMPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC VM PID: ", err)
	}

	if w.wilcoDTCVMPID != pid {
		s.Error("The Wilco DTC VM PID changed while testing")
	}

	pid, err = wilco.SupportdPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC Support Daemon PID: ", err)
	}

	if w.wilcoDTCSupportdPID != pid {
		s.Error("The Wilco DTC Support Daemon PID changed while testing")
	}
}

func (w *wilcoEnrolledFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Stopping the the Wilco DTC VM and Daemon.
	if err := wilco.StopSupportd(ctx); err != nil {
		s.Error("Failed to stop the Wilco DTC Support Daemon: ", err)
	}

	if err := wilco.StopVM(ctx); err != nil {
		s.Error("Failed to stop the Wilco DTC VM: ", err)
	}

	if w.onlyFakeDMS {
		if err := w.cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}

	if w.extensionDir != "" {
		if err := os.RemoveAll(w.extensionDir); err != nil {
			s.Error("Failed to remove Chrome extension dir: ", err)
		}
	}
}

func (w *wilcoEnrolledFixture) Reset(ctx context.Context) error {
	if !w.onlyFakeDMS {
		return nil
	}

	// Check the connection to Chrome.
	if err := w.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	// Reset Chrome state.
	if err := w.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}

	return nil
}
