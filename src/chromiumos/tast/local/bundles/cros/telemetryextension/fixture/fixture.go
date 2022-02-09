// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture contains Telemetry Extension fixture.
package fixture

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	manifestJSON = "manifest.json"
	swJS         = "sw.js"

	cleanupTimeout = chrome.ResetTimeout + 20*time.Second
)

var dataFiles = []string{manifestJSON, swJS}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "telemetryExtension",
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(false),
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		Data:            dataFiles,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "telemetryExtensionManaged",
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension for managed device",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(true),
		Parent:          fixture.Enrolled,
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		Data:            dataFiles,
		// Vars:            []string{"policy.ExtensionInstallForceList.username", "policy.ExtensionInstallForceList.password"},
		// Vars:            []string{"policy.GAIAReporting.user_name", "policy.GAIAReporting.password"},
		Vars: []string{"telemetryextension.Fixture.username", "telemetryextension.Fixture.password"},
	})
}

func newTelemetryExtensionFixture(managed bool) *telemetryExtensionFixture {
	return &telemetryExtensionFixture{
		managed: managed,
	}
}

// telemetryExtensionFixture implements testing.FixtureImpl.
type telemetryExtensionFixture struct {
	managed bool

	dir string
	cr  *chrome.Chrome

	v Value
}

// Value is a value exposed by fixture to tests.
type Value struct {
	ExtID string

	PwaConn *chrome.Conn
	ExtConn *chrome.Conn
}

func (f *telemetryExtensionFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cleanupCtx, cancel := ctxutil.Shorten(ctx, cleanupTimeout)
	defer cancel()

	defer func(ctx context.Context) {
		if s.HasError() {
			f.cleanUp(ctx, s)
		}
	}(cleanupCtx)

	if !f.managed {
		dir, err := ioutil.TempDir("", "telemetry_extension")
		if err != nil {
			s.Fatal("Failed to create temporary directory for TelemetryExtension: ", err)
		}
		f.dir = dir

		if err := os.Chown(dir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
			s.Fatal("Failed to chown TelemetryExtension dir: ", err)
		}

		for _, file := range dataFiles {
			if err := fsutil.CopyFile(s.DataPath(file), filepath.Join(dir, file)); err != nil {
				s.Fatalf("Failed to copy %q file to %q: %v", file, dir, err)
			}

			if err := os.Chown(filepath.Join(dir, file), int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
				s.Fatalf("Failed to chown %q: %v", file, err)
			}
		}

		cr, err := chrome.New(ctx, chrome.UnpackedExtension(dir))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		f.cr = cr
	} else {
		// fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
		// if !ok {
		// 	s.Fatal("Parent is not a FakeDMS fixture")
		// }

		// The user has the ExtensionInstallForceList policy set.
		// username := s.RequiredVar("policy.ExtensionInstallForceList.username")
		// password := s.RequiredVar("policy.ExtensionInstallForceList.password")

		// username := s.RequiredVar("policy.GAIAReporting.user_name")
		// password := s.RequiredVar("policy.GAIAReporting.password")

		username := s.RequiredVar("telemetryextension.Fixture.username")
		password := s.RequiredVar("telemetryextension.Fixture.password")

		s.Log("Creds: ", username, password)

		cr, err := chrome.New(ctx,
			chrome.KeepEnrollment(),
			// chrome.GAIAEnterpriseEnroll(chrome.Creds{User: username, Pass: password}),
			chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
			// chrome.ProdPolicy(),
			// chrome.DMSPolicy(fdms.URL),
			// chrome.ExtraArgs("--login-manager"),
			chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout))
		if err != nil {
			s.Fatal("Chrome startup failed: ", err)
		}
		f.cr = cr

		// pb := fakedms.NewPolicyBlob()
		// // Telemetry Extension work only for affiliated users.
		// pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
		// pb.UserAffiliationIds = []string{"default_affiliation_id"}
		// pb.AddPolicy(&policy.ExtensionInstallForcelist{Val: []string{
		// 	"gogonhoemckpdpadfnjnpgbjpbjnodgc;https://clients2.google.com/service/update2/crx",
		// }})

		// if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		// 	s.Fatal("Failed to serve and refresh: ", err)
		// }
	}

	pwaConn, err := f.cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create connection to google.com: ", err)
	}
	f.v.PwaConn = pwaConn

	if err := chrome.AddTastLibrary(ctx, pwaConn); err != nil {
		s.Fatal("Failed to add Tast library to google.com: ", err)
	}

	f.v.ExtID = "gogonhoemckpdpadfnjnpgbjpbjnodgc"

	extConn, err := f.cr.NewConn(ctx, fmt.Sprintf("chrome-extension://%s/sw.js", f.v.ExtID))
	if err != nil {
		s.Fatal("Failed to create connection to Telemetry Extension: ", err)
	}
	f.v.ExtConn = extConn

	if err := chrome.AddTastLibrary(ctx, extConn); err != nil {
		s.Fatal("Failed to add Tast library to Telemetry Extension: ", err)
	}

	time.Sleep(60 * time.Second)

	return &f.v
}

func (f *telemetryExtensionFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	f.cleanUp(ctx, s)
}

func (f *telemetryExtensionFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *telemetryExtensionFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *telemetryExtensionFixture) Reset(ctx context.Context) error {
	return nil
}

// cleanUp releases all associated resources with fixture.
func (f *telemetryExtensionFixture) cleanUp(ctx context.Context, s *testing.FixtState) {
	if f.v.ExtConn != nil {
		if err := f.v.ExtConn.Close(); err != nil {
			s.Error("Failed to close connection to Telemetry Extension: ", err)
		}
		f.v.ExtConn = nil
	}

	if f.v.PwaConn != nil {
		if err := f.v.PwaConn.Close(); err != nil {
			s.Error("Failed to close connection to google.com: ", err)
		}
		f.v.PwaConn = nil
	}

	if f.cr != nil {
		if err := f.cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome: ", err)
		}
		f.cr = nil
	}

	if f.dir != "" {
		if err := os.RemoveAll(f.dir); err != nil {
			s.Error("Failed to remove directory with Telemetry Extension: ", err)
		}
		f.dir = ""
	}
}
