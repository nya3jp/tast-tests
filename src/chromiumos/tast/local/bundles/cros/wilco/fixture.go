// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wilco/wilcoextension"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "wilcoDTCAllowed",
		Desc: "Wilco DTC fixture with support for DTC VM, Supportd daemon and Wilco Chrome Extension",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Impl:            NewWilcoFixture(false),
		Parent:          fixture.FakeDMSEnrolled,
		SetUpTimeout:    chrome.ManagedUserLoginTimeout + 30*time.Second,
		PostTestTimeout: 15 * time.Second,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout + 20*time.Second,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "wilcoDTCAllowedVMTestMode",
		Desc: "Wilco DTC fixture with support for DTC VM (Test Mode Configuration), Supportd daemon and Wilco Chrome Extension",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Impl:            NewWilcoFixture(true),
		Parent:          fixture.FakeDMSEnrolled,
		SetUpTimeout:    chrome.ManagedUserLoginTimeout + time.Minute,
		PostTestTimeout: 15 * time.Second,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout + 20*time.Second,
	})
}

// NewWilcoFixture returns a wilcoDTCFixture object reference that implements FixtureImpl interface.
func NewWilcoFixture(vmTestMode bool) *wilcoDTCFixture {
	return &wilcoDTCFixture{
		launchVMInTestMode: vmTestMode,
	}
}

// wilcoDTCFixture implements testing.FixtureImpl.
type wilcoDTCFixture struct {
	// launchVMInTestMode, if enabled, relaunches VM in test configuration.
	launchVMInTestMode bool

	// To enforce the checking of vm and supportd instances don't get changed by the test code.
	wilcoDTCVMPID       int
	wilcoDTCSupportdPID int

	// Local chrome & fdms state.
	cr   *chrome.Chrome
	fdms *fakedms.FakeDMS

	// Extension directory where assets are stored temporarily.
	extensionDir string
}

func (w *wilcoDTCFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	// Load wilco extension alongside chrome.
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

	pb := policy.NewBlob()
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

	// Ensuring the VM is ready to run commands over vsh.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return vm.CreateVSHCommand(ctx, wilco.WilcoVMCID, "true").Run()
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to wait for DTC VM to be ready: ", err)
	}

	// Verify that wilco_dtc_supportd daemon was started by policy.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := wilco.SupportdPID(ctx); err != nil {
			return errors.Wrap(err, "failed to get Wilco DTC Support daemon PID")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for Wilco DTC Support Daemon to start: ", err)
	}

	// Restart wilco_dtc_supportd daemon in a test mode to collect more verbose logs.
	if err := wilco.StartSupportd(ctx); err != nil {
		s.Fatal("Failed to restart Wilco DTC Support Daemon: ", err)
	}

	w.wilcoDTCSupportdPID, err = wilco.SupportdPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC Support Daemon PID after daemon restart: ", err)
	}

	// If wilco VM needs to be in test mode. Restarting wilco DTC vm with updated config.
	if w.launchVMInTestMode {
		if err := wilco.StopVM(ctx); err != nil {
			s.Fatal("Failed to stop DTC VM: ", err)
		}
		if err := wilco.StartVM(ctx, &wilco.VMConfig{
			StartProcesses: false,
			TestDBusConfig: false,
		}); err != nil {
			s.Fatal("Failed to start the Wilco DTC VM: ", err)
		}
	}

	w.wilcoDTCVMPID, err = wilco.VMPID(ctx)
	if err != nil {
		s.Fatal("Failed to get Wilco DTC VM PID: ", err)
	}

	// Wait until wilco_dtc_supportd bootstrapped the Mojo connection to Chrome.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		resp := dtcpb.GetStatefulPartitionAvailableCapacityResponse{}
		if err := wilco.DPSLSendMessage(ctx, "GetStatefulPartitionAvailableCapacity",
			&dtcpb.GetStatefulPartitionAvailableCapacityRequest{}, &resp); err != nil {
			return errors.Wrap(err, "failed to get stateful partition available capacity")
		}
		if want := dtcpb.GetStatefulPartitionAvailableCapacityResponse_STATUS_OK; resp.Status != want {
			return errors.Errorf("unexpected status received from vsh rpc method call = got %v, want %v", resp.Status, want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait wilco_dtc_supportd to bootstrap the Mojo connection to Chrome: ", err)
	}

	return fixtures.NewFixtData(w.cr, w.fdms)
}

func (w *wilcoDTCFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (w *wilcoDTCFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
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
		s.Fatal("Failed to get Wilco DTC Support daemon PID: ", err)
	}

	if w.wilcoDTCSupportdPID != pid {
		s.Error("The Wilco DTC Support Daemon PID changed while testing")
	}
}

func (w *wilcoDTCFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Stopping the the Wilco DTC VM and Daemon.
	if err := wilco.StopSupportd(ctx); err != nil {
		s.Error("Failed to stop the Wilco DTC Support Daemon: ", err)
	}

	if err := wilco.StopVM(ctx); err != nil {
		s.Error("Failed to stop the Wilco DTC VM: ", err)
	}

	if err := w.cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	if err := os.RemoveAll(w.extensionDir); err != nil {
		s.Error("Failed to remove Chrome extension dir: ", err)
	}
}

func (w *wilcoDTCFixture) Reset(ctx context.Context) error {
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
