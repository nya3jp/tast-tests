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
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	cleanupTimeout = chrome.ResetTimeout + 20*time.Second

	crosHealthdJobName = "cros_healthd"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "telemetryExtension",
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(),
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
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
		Impl:            newTelemetryExtensionFixture(optionsPage()),
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
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
		Impl:            newTelemetryExtensionFixture(managed()),
		Parent:          fixture.FakeDMSEnrolled,
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
		Vars:            []string{"policy.ManagedUser.accountPool"},
	})
}

func manifestFile(optionsPage bool) string {
	if optionsPage {
		return "manifest_with_options_page.json"
	}
	return "manifest_without_options_page.json"
}

func extFiles(optionsPage bool) []string {
	files := []string{manifestFile(optionsPage), "sw.js"}
	if optionsPage {
		files = append(files, "options.html")
	}
	return files
}

type option func(*telemetryExtensionFixture)

func optionsPage() func(*telemetryExtensionFixture) {
	return func(f *telemetryExtensionFixture) {
		f.optionsPage = true
	}
}

func managed() func(*telemetryExtensionFixture) {
	return func(f *telemetryExtensionFixture) {
		f.managed = true
	}
}

func newTelemetryExtensionFixture(opts ...option) *telemetryExtensionFixture {
	f := &telemetryExtensionFixture{}
	f.v.ExtID = "gogonhoemckpdpadfnjnpgbjpbjnodgc"

	for _, opt := range opts {
		opt(f)
	}
	return f
}

// telemetryExtensionFixture implements testing.FixtureImpl.
type telemetryExtensionFixture struct {
	optionsPage bool
	managed     bool

	dir string
	cr  *chrome.Chrome

	healthdPID int

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

	if f.managed {
		fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
		if !ok {
			s.Fatal("Parent is not a FakeDMS fixture")
		}

		gaiaCreds, err := chrome.PickRandomCreds(s.RequiredVar("policy.ManagedUser.accountPool"))
		if err != nil {
			s.Fatal("Failed to parse managed user creds: ", err)
		}

		if err := f.setupChromeForManagedUsers(ctx, fdms, gaiaCreds.User, gaiaCreds.Pass); err != nil {
			s.Fatal("Failed to setup Chrome for managed users: ", err)
		}
	} else {
		if err := f.setupChromeForConsumers(ctx, s.DataPath); err != nil {
			s.Fatal("Failed to setup Chrome for consumers: ", err)
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

	if err := upstart.EnsureJobRunning(ctx, crosHealthdJobName); err != nil {
		s.Fatalf("Failed to start %s daemon", crosHealthdJobName)
	}

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

func (f *telemetryExtensionFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	_, _, pid, err := upstart.JobStatus(ctx, crosHealthdJobName)
	if err != nil {
		s.Fatalf("Unable to get %s PID: %s", crosHealthdJobName, err)
	}

	f.healthdPID = pid
}

func (f *telemetryExtensionFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	_, _, pid, err := upstart.JobStatus(ctx, crosHealthdJobName)
	if err != nil {
		s.Fatalf("Unable to get %s PID: %s", crosHealthdJobName, err)
	}

	if pid != f.healthdPID {
		s.Fatalf("%s PID changed: got %d, want %d", crosHealthdJobName, pid, f.healthdPID)
	}
}

func (f *telemetryExtensionFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *telemetryExtensionFixture) setupChromeForConsumers(ctx context.Context, dataPathFunc func(string) string) error {
	dir, err := ioutil.TempDir("", "telemetry_extension")
	if err != nil {
		return errors.Wrap(err, "failed to create temporary directory for TelemetryExtension")
	}
	f.dir = dir

	if err := os.Chown(dir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return errors.Wrap(err, "failed to chown TelemetryExtension dir")
	}

	for _, file := range extFiles(f.optionsPage) {
		if err := fsutil.CopyFile(dataPathFunc(file), filepath.Join(dir, file)); err != nil {
			return errors.Wrapf(err, "failed to copy %q file to %q", file, dir)
		}

		if err := os.Chown(filepath.Join(dir, file), int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
			return errors.Wrapf(err, "failed to chown %q", file)
		}
	}

	if err := os.Rename(filepath.Join(dir, manifestFile(f.optionsPage)), filepath.Join(dir, "manifest.json")); err != nil {
		return errors.Wrap(err, "failed to rename manifest file")
	}

	cr, err := chrome.New(ctx, chrome.UnpackedExtension(dir))
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	f.cr = cr
	return nil
}

func (f *telemetryExtensionFixture) setupChromeForManagedUsers(ctx context.Context, fdms *fakedms.FakeDMS, username, password string) error {
	pb := policy.NewBlob()
	pb.PolicyUser = username

	// Telemetry Extension works only for affiliated users.
	pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
	pb.UserAffiliationIds = []string{"default_affiliation_id"}

	// We have to update fake DMS policy user and affiliation IDs before starting Chrome.
	if err := fdms.WritePolicyBlob(pb); err != nil {
		return errors.Wrap(err, "failed to write policy blob before starting Chrome")
	}

	cr, err := chrome.New(ctx,
		chrome.KeepEnrollment(),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout))
	if err != nil {
		return errors.Wrap(err, "Chrome startup failed")
	}
	f.cr = cr

	// Force install Telemetry Extension by policy.
	pb.AddPolicy(&policy.ExtensionInstallForcelist{Val: []string{f.v.ExtID}})
	// Allow DevTools on force installed extensions. Value 1 here means "allowed".
	pb.AddPolicy(&policy.DeveloperToolsAvailability{Val: 1})

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		return errors.Wrap(err, "failed to serve and refresh")
	}
	return nil
}
