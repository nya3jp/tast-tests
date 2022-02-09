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
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const cleanupTimeout = chrome.ResetTimeout + 20*time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "telemetryExtension",
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(false, false),
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		Data:            extFiles(false),
	})
	testing.AddFixture(&testing.Fixture{
		Name: "telemetryExtensionOptionsPage",
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension with options page",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(true, false),
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		Data:            extFiles(true),
	})
	testing.AddFixture(&testing.Fixture{
		Name: "telemetryExtensionManaged",
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension for managed device",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(false, true),
		Parent:          fixture.FakeDMSEnrolled,
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		Vars:            []string{"telemetryextension.Fixture.username", "telemetryextension.Fixture.password"},
	})
}

func manifestFile(hasOptions bool) string {
	if hasOptions {
		return "manifest_with_options_page.json"
	}
	return "manifest_without_options_page.json"
}

func extFiles(hasOptions bool) []string {
	files := []string{manifestFile(hasOptions), "sw.js"}
	if hasOptions {
		files = append(files, "options.html")
	}
	return files
}

func newTelemetryExtensionFixture(hasOptions bool, isManaged bool) *telemetryExtensionFixture {
	return &telemetryExtensionFixture{
		hasOptions: hasOptions,
		isManaged:  isManaged,
	}
}

// telemetryExtensionFixture implements testing.FixtureImpl.
type telemetryExtensionFixture struct {
	hasOptions bool
	isManaged  bool

	dir string
	cr  *chrome.Chrome

	v Value
}

// Value is a value exposed by fixture to tests.
type Value struct {
	ExtID string

	PwaConn *chrome.Conn
	ExtConn *chrome.Conn

	TConn *chrome.TestConn
}

func (f *telemetryExtensionFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cleanupCtx, cancel := ctxutil.Shorten(ctx, cleanupTimeout)
	defer cancel()

	defer func(ctx context.Context) {
		if s.HasError() {
			f.TearDown(ctx, s)
		}
	}(cleanupCtx)

	if !f.isManaged {
		dir, err := ioutil.TempDir("", "telemetry_extension")
		if err != nil {
			s.Fatal("Failed to create temporary directory for TelemetryExtension: ", err)
		}
		f.dir = dir

		if err := os.Chown(dir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
			s.Fatal("Failed to chown TelemetryExtension dir: ", err)
		}

		for _, file := range extFiles(f.hasOptions) {
			if err := fsutil.CopyFile(s.DataPath(file), filepath.Join(dir, file)); err != nil {
				s.Fatalf("Failed to copy %q file to %q: %v", file, dir, err)
			}

			if err := os.Chown(filepath.Join(dir, file), int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
				s.Fatalf("Failed to chown %q: %v", file, err)
			}
		}

		if err := os.Rename(filepath.Join(dir, manifestFile(f.hasOptions)), filepath.Join(dir, "manifest.json")); err != nil {
			s.Fatal("Failed to rename manifest file: ", err)
		}

		cr, err := chrome.New(ctx, chrome.UnpackedExtension(dir))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		f.cr = cr
	} else {
		fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
		if !ok {
			s.Fatal("Parent is not a FakeDMS fixture")
		}

		username := s.RequiredVar("telemetryextension.Fixture.username")
		password := s.RequiredVar("telemetryextension.Fixture.password")

		pb := fakedms.NewPolicyBlob()
		pb.PolicyUser = username

		// Telemetry Extension works only for affiliated users.
		pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
		pb.UserAffiliationIds = []string{"default_affiliation_id"}

		// We have to update fake DMS policy user and affiliation IDs before starting Chrome.
		if err := fdms.WritePolicyBlob(pb); err != nil {
			s.Fatal("Failed to write policy blob before starting Chrome: ", err)
		}

		cr, err := chrome.New(ctx,
			chrome.KeepEnrollment(),
			chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
			chrome.DMSPolicy(fdms.URL),
			chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout))
		if err != nil {
			s.Fatal("Chrome startup failed: ", err)
		}
		f.cr = cr

		// Force install Telemetry Extension by policy.
		pb.AddPolicy(&policy.ExtensionInstallForcelist{Val: []string{
			"gogonhoemckpdpadfnjnpgbjpbjnodgc;https://clients2.google.com/service/update2/crx",
		}})
		// Allow DevTools on force installed extensions. Value 1 here means "allowed".
		pb.AddPolicy(&policy.DeveloperToolsAvailability{Val: 1})

		if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
			s.Fatal("Failed to serve and refresh: ", err)
		}
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

	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connections: ", err)
	}
	f.v.TConn = tconn

	return &f.v
}

func (f *telemetryExtensionFixture) TearDown(ctx context.Context, s *testing.FixtState) {
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

func (f *telemetryExtensionFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *telemetryExtensionFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *telemetryExtensionFixture) Reset(ctx context.Context) error {
	return nil
}
