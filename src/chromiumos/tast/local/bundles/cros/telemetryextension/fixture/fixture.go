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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	manifestJSON = "manifest.json"
	optionsHTML  = "options.html"
	swJS         = "sw.js"

	cleanupTimeout = chrome.ResetTimeout + 20*time.Second
)

var dataFiles = []string{manifestJSON, optionsHTML, swJS}

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
		Data:            dataFiles,
	})
}

func newTelemetryExtensionFixture() *telemetryExtensionFixture {
	return &telemetryExtensionFixture{}
}

// telemetryExtensionFixture implements testing.FixtureImpl.
type telemetryExtensionFixture struct {
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
			f.TearDown(ctx, s)
		}
	}(cleanupCtx)

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

	pwaConn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create connection to google.com: ", err)
	}
	f.v.PwaConn = pwaConn

	if err := chrome.AddTastLibrary(ctx, pwaConn); err != nil {
		s.Fatal("Failed to add Tast library to google.com: ", err)
	}

	f.v.ExtID = "gogonhoemckpdpadfnjnpgbjpbjnodgc"

	extConn, err := cr.NewConn(ctx, fmt.Sprintf("chrome-extension://%s/sw.js", f.v.ExtID))
	if err != nil {
		s.Fatal("Failed to create connection to Telemetry Extension: ", err)
	}
	f.v.ExtConn = extConn

	if err := chrome.AddTastLibrary(ctx, extConn); err != nil {
		s.Fatal("Failed to add Tast library to Telemetry Extension: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connections: ", err)
	}

	if err := requestSerialNumber(ctx, extConn, tconn); err != nil {
		s.Fatal("Failed to request serial number permission from options page: ", err)
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

func (f *telemetryExtensionFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *telemetryExtensionFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *telemetryExtensionFixture) Reset(ctx context.Context) error {
	return nil
}

// requestSerialNumber opens options page and requests os.telemetry.serial_number permission.
func requestSerialNumber(ctx context.Context, conn *chrome.Conn, tconn *chrome.TestConn) error {
	if err := conn.Call(ctx, nil,
		"tast.promisify(chrome.runtime.openOptionsPage)",
	); err != nil {
		return errors.Wrap(err, "failed to get response from Telemetry Extenion service worker")
	}

	ui := uiauto.New(tconn)
	requestButton := nodewith.Name("Add serial number permission").Role(role.Button)
	allowButton := nodewith.Name("Allow").Role(role.Button)
	if err := uiauto.Combine("allow serial number permission",
		ui.WithTimeout(5*time.Second).WaitUntilExists(requestButton),
		ui.LeftClick(requestButton),
		ui.WithTimeout(5*time.Second).WaitUntilExists(allowButton),
		ui.LeftClickUntil(allowButton, ui.Gone(allowButton)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to allow serial number permission")
	}
	return nil
}
