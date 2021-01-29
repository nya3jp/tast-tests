// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
  "context"
  "time"

  "chromiumos/tast/local/chrome"
  "chromiumos/tast/local/chrome/ui"
  "chromiumos/tast/local/chrome/ui/quicksettings"
  "chromiumos/tast/ctxutil"
  "chromiumos/tast/testing"
)

func init() {
  testing.AddTest(&testing.Test{
    Func: QuickSettingsManagedDeviceInfo,
    Desc: "Checks that the Quick Settings managed device info is displayed correctly",
    Contacts: []string{
      "leandre@chromium.org",
      "tbarzic@chromium.org",
      "kaznacheev@chromium.org",
    },
    Attr:         []string{"group:mainline", "informational"},
    SoftwareDeps: []string{"chrome"},
    Vars:         []string{"ui.QuickSettingsManagedDeviceInfo.user", "ui.QuickSettingsManagedDeviceInfo.password"},
  })
}

const uiTimeout = 10 * time.Second

// QuickSettingsManagedDeviceInfo tests that the Quick Settings managed device info is displayed correctly.
func QuickSettingsManagedDeviceInfo(ctx context.Context, s *testing.State) {
  const (
    cleanupTime = 10 * time.Second // time reserved for cleanup.
  )
  username := s.RequiredVar("ui.QuickSettingsManagedDeviceInfo.user")
  password := s.RequiredVar("ui.QuickSettingsManagedDeviceInfo.password")

 cr, err := chrome.New(
   ctx,
   chrome.Auth(username, password, "gaia-id"),
   chrome.GAIALogin(),
   chrome.EnableFeatures("ManagedDeviceUIRedesign"),
   chrome.ProdPolicy())
 if err != nil {
   s.Fatal("Failed to connect to Chrome: ", err)
 }
 defer cr.Close(ctx)

 // Use a shorter context to leave time for cleanup.
  ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
  defer cancel()

  tconn, err := cr.TestAPIConn(ctx)
  if err != nil {
    s.Fatal("Failed to create Test API connection: ", err)
  }

  if err := quicksettings.Show(ctx, tconn); err != nil {
    s.Fatal("Failed to show Quick Settings: ", err)
  }
  defer quicksettings.Hide(ctx, tconn)

  params := ui.FindParams{
    ClassName: "UnifiedManagedDeviceView",
  }

  managedBtn, err := ui.FindWithTimeout(ctx, tconn, params, uiTimeout)
  if err != nil {
    s.Fatal("failed to find managed info button", err)
  }

  // Try clicking the Settings button until it goes away, indicating the click was received.
  // todo(crbug/1099502): determine when this is clickable, and just click it once.
  condition := func(ctx context.Context) (bool, error) {
    exists, err := ui.Exists(ctx, tconn, params)
    return !exists, err
  }
  opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}
  if err := managedBtn.LeftClickUntil(ctx, condition, &opts); err != nil {
    s.Fatal("managed info button still present after clicking", err)
  }
}
