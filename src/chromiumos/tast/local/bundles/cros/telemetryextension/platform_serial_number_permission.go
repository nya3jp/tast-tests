// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"reflect"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/telemetryextension/dep"
	"chromiumos/tast/local/bundles/cros/telemetryextension/fixture"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlatformSerialNumberPermission,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests that Chrome extension can have options page and request additional serial number permission at runtime",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name:              "stable",
				Fixture:           fixture.TelemetryExtensionOptionsPage,
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable",
				Fixture:           fixture.TelemetryExtensionOptionsPage,
				ExtraHardwareDeps: dep.NonStableModels(),
			},
			{
				Name:              "stable_lacros",
				Fixture:           fixture.TelemetryExtensionOptionsPageLacros,
				ExtraHardwareDeps: dep.StableModels(),
			},
			{
				Name:              "non_stable_lacros",
				Fixture:           fixture.TelemetryExtensionOptionsPageLacros,
				ExtraHardwareDeps: dep.NonStableModels(),
			},
		},
	})
}

// PlatformSerialNumberPermission tests Chrome extension can have options page and request additional serial number permission at runtime.
// TODO(b/246764355): stop using fixture for this test.
func PlatformSerialNumberPermission(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)

	if err := checkPermissions(ctx, v.ExtConn); err != nil {
		s.Fatal("Failed to check permissions before requesting additional permission: ", err)
	}

	if err := requestSerialNumber(ctx, v.ExtConn, v.TConn); err != nil {
		s.Fatal("Failed to request serial number permission from options page: ", err)
	}

	if err := checkPermissions(ctx, v.ExtConn, "os.telemetry.serial_number"); err != nil {
		s.Fatal("Failed to check permissions after requesting additional permission: ", err)
	}
}

// checkPermissions checks whether Chrome extension has expected permissions.
func checkPermissions(ctx context.Context, conn *chrome.Conn, wantPermissions ...string) error {
	type response struct {
		Permissions []string `json:"permissions"`
	}

	var resp response
	if err := conn.Call(ctx, &resp,
		"tast.promisify(chrome.permissions.getAll)",
	); err != nil {
		return errors.Wrap(err, "failed to get response from Telemetry Extenion service worker")
	}

	// reflect.DeepEqual(nil, []string{}) returns false, so handle it separately.
	if wantPermissions == nil {
		wantPermissions = []string{}
	}

	if !reflect.DeepEqual(resp.Permissions, wantPermissions) {
		return errors.Errorf("unexpected permissions = got %v, want %v, want is nil %t", resp.Permissions, wantPermissions, wantPermissions == nil)
	}

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
	if err := uiauto.Combine("request serial number permission",
		ui.WithTimeout(5*time.Second).WaitUntilExists(requestButton),
		ui.LeftClick(requestButton),
		ui.WithTimeout(5*time.Second).WaitUntilExists(allowButton),
		ui.LeftClickUntil(allowButton, ui.Gone(allowButton)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to request serial number permission")
	}
	return nil
}
