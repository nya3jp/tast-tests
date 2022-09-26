// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/bundles/cros/telemetryextension/vendorutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Fixture names.
const (
	TelemetryExtension                             = "telemetryExtension"
	TelemetryExtensionLacros                       = "telemetryExtensionLacros"
	TelemetryExtensionOverrideOEMName              = "telemetryExtensionOverrideOEMName"
	TelemetryExtensionOverrideOEMNameLacros        = "telemetryExtensionOverrideOEMNameLacros"
	TelemetryExtensionOptionsPage                  = "telemetryExtensionOptionsPage"
	TelemetryExtensionOptionsPageLacros            = "telemetryExtensionOptionsPageLacros"
	TelemetryExtensionManaged                      = "telemetryExtensionManaged"
	TelemetryExtensionManagedLacros                = "telemetryExtensionManagedLacros"
	TelemetryExtensionOverrideOEMNameManaged       = "telemetryExtensionOverrideOEMNameManaged"
	TelemetryExtensionOverrideOEMNameManagedLacros = "telemetryExtensionOverrideOEMNameManagedLacros"
)

const (
	cleanupTimeout = chrome.ResetTimeout + 20*time.Second

	crosHealthdJobName = "cros_healthd"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: TelemetryExtension,
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
		Name: TelemetryExtensionLacros,
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension in Lacros browser",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(lacros()),
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
		Data:            extFiles(false),
	})
	testing.AddFixture(&testing.Fixture{
		Name: TelemetryExtensionOverrideOEMName,
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension on devices that are not officially supported yet",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(overrideOEMName()),
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
		Data:            extFiles(false),
	})
	testing.AddFixture(&testing.Fixture{
		Name: TelemetryExtensionOverrideOEMNameLacros,
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension in Lacros browser on devices that are not officially supported yet",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(lacros(), overrideOEMName()),
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
		Data:            extFiles(false),
	})
	testing.AddFixture(&testing.Fixture{
		Name: TelemetryExtensionOptionsPage,
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
		Name: TelemetryExtensionOptionsPageLacros,
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension with options page in Lacros browser",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(lacros(), optionsPage()),
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
		Data:            extFiles(true),
	})
	testing.AddFixture(&testing.Fixture{
		Name: TelemetryExtensionManaged,
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension on managed device",
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
	testing.AddFixture(&testing.Fixture{
		Name: TelemetryExtensionManagedLacros,
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension in Lacros browser on managed device",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(managed(), lacros()),
		Parent:          fixture.PersistentLacrosEnrolled,
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
		Vars:            []string{"policy.ManagedUser.accountPool"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: TelemetryExtensionOverrideOEMNameManaged,
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension on managed devices that are not officially supported yet",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(overrideOEMName(), managed()),
		Parent:          fixture.FakeDMSEnrolled,
		SetUpTimeout:    chrome.LoginTimeout + 30*time.Second + cleanupTimeout,
		TearDownTimeout: cleanupTimeout,
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 10 * time.Second,
		Vars:            []string{"policy.ManagedUser.accountPool"},
	})
	testing.AddFixture(&testing.Fixture{
		Name: TelemetryExtensionOverrideOEMNameManagedLacros,
		Desc: "Telemetry Extension fixture with running PWA and companion Telemetry Extension in Lacros browser on managed devices that are not officially supported yet",
		Contacts: []string{
			"lamzin@google.com", // Fixture and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Impl:            newTelemetryExtensionFixture(overrideOEMName(), managed(), lacros()),
		Parent:          fixture.PersistentLacrosEnrolled,
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

func lacros() func(*telemetryExtensionFixture) {
	return func(f *telemetryExtensionFixture) {
		f.bt = browser.TypeLacros
	}
}

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

func overrideOEMName() func(*telemetryExtensionFixture) {
	return func(f *telemetryExtensionFixture) {
		f.overrideOEMName = true
	}
}

func newTelemetryExtensionFixture(opts ...option) *telemetryExtensionFixture {
	f := &telemetryExtensionFixture{}
	f.bt = browser.TypeAsh
	f.v.ExtID = "gogonhoemckpdpadfnjnpgbjpbjnodgc"

	for _, opt := range opts {
		opt(f)
	}
	return f
}

// telemetryExtensionFixture implements testing.FixtureImpl.
type telemetryExtensionFixture struct {
	bt              browser.Type
	optionsPage     bool
	managed         bool
	overrideOEMName bool

	dir     string
	cr      *chrome.Chrome
	br      *browser.Browser
	closeBr uiauto.Action

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

	if f.bt == browser.TypeLacros {
		// TODO(b/245337406): Find fix for the actual error instead
		// of this workaround.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Unable to pause between Ash and Lacros launch")
		}
	}

	br, closeBr, err := browserfixt.SetUp(ctx, f.cr, f.bt)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	f.br = br
	f.closeBr = closeBr

	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connections: ", err)
	}
	f.v.TConn = tconn

	if err := f.setupConnectionToPWA(ctx); err != nil {
		s.Fatal("Failed to setup connection to PWA: ", err)
	}

	if err := f.setupConnectionToExtension(ctx); err != nil {
		s.Fatal("Failed to setup connection to Telemetry Extension: ", err)
	}

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

	if f.br != nil {
		f.closeBr(ctx)

		f.br = nil
		f.closeBr = nil
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

	var opts []chrome.Option
	if err := f.addOverrideOEMNameChromeArg(ctx, &opts); err != nil {
		return err
	}
	if f.bt == browser.TypeAsh {
		opts = append(opts, chrome.UnpackedExtension(dir))
	}
	if f.bt == browser.TypeLacros {
		extraOpts, err := lacrosfixt.NewConfig().Opts()
		if err != nil {
			return errors.Wrap(err, "failed to get lacros options")
		}
		opts = append(opts, extraOpts...)
		opts = append(opts, chrome.LacrosUnpackedExtension(dir))
	}

	cr, err := chrome.New(ctx, opts...)
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

	opts := []chrome.Option{
		chrome.KeepEnrollment(),
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout),
	}
	if err := f.addOverrideOEMNameChromeArg(ctx, &opts); err != nil {
		return err
	}
	if f.bt == browser.TypeLacros {
		extraOpts, err := lacrosfixt.NewConfig(
			lacrosfixt.ChromeOptions(chrome.GAIALogin(chrome.Creds{User: username, Pass: password}))).Opts()
		if err != nil {
			return errors.Wrap(err, "failed to get lacros options")
		}
		opts = append(opts, extraOpts...)
	}

	cr, err := chrome.New(ctx, opts...)
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

func (f *telemetryExtensionFixture) setupConnectionToPWA(ctx context.Context) error {
	pwaConn, err := f.br.NewConn(ctx, "https://googlechromelabs.github.io/cros-sample-telemetry-extension")
	if err != nil {
		return errors.Wrap(err, "failed to create connection to googlechromelabs.github.io")
	}
	f.v.PwaConn = pwaConn

	if err := chrome.AddTastLibrary(ctx, pwaConn); err != nil {
		return errors.Wrap(err, "failed to add Tast library to googlechromelabs.github.io")
	}
	return nil
}

func (f *telemetryExtensionFixture) setupConnectionToExtension(ctx context.Context) error {
	conn, err := f.br.NewConn(ctx, fmt.Sprintf("chrome-extension://%s/sw.js", f.v.ExtID))
	if err != nil {
		return errors.Wrap(err, "failed to create connection to Telemetry Extension")
	}
	f.v.ExtConn = conn

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		const js = `
			(function() {
				return document.querySelector("body > pre") !== null;
			})()
		`
		isExtensionInstalled := false
		if err := conn.Eval(ctx, js, &isExtensionInstalled); err != nil {
			return errors.Wrap(err, "failed to verify whether chrome.os.telemetry is defined")
		}

		if isExtensionInstalled {
			return nil
		}

		reloadButton := nodewith.Role(role.Button).Name("Reload").ClassName("ReloadButton")
		if err := uiauto.New(f.v.TConn).LeftClick(reloadButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to click reload button")
		}

		return errors.New("chrome.os.telemetry is undefined")
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify that extension is installed")
	}

	if err := chrome.AddTastLibrary(ctx, conn); err != nil {
		return errors.Wrap(err, "failed to add Tast library to Telemetry Extension")
	}

	return nil
}

func (f *telemetryExtensionFixture) addOverrideOEMNameChromeArg(ctx context.Context, opts *[]chrome.Option) error {
	if f.overrideOEMName {
		vendorName, err := vendorutils.FetchVendor(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to fetch vendor name")
		}
		*opts = append(*opts, chrome.ExtraArgs("--telemetry-extension-manufacturer-override-for-testing="+vendorName))
	}
	return nil
}
