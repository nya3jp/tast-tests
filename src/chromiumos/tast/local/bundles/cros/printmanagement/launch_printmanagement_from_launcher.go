// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printmanagement

import (
        "context"
        "time"

        "chromiumos/tast/ctxutil"
        "chromiumos/tast/local/apps"
        "chromiumos/tast/local/chrome"
        "chromiumos/tast/local/chrome/ui/faillog"
        "chromiumos/tast/local/chrome/ui/launcher"
        "chromiumos/tast/local/chrome/ui/printmanagementapp"
        "chromiumos/tast/testing"
)

func init() {
        testing.AddTest(&testing.Test{
                Func: LaunchPrintmanagementFromLauncher,
                Desc: "Print management app can be found and launched from the launcher",
                Contacts: []string{
                        "michaelcheco@google.com",
                        "cros-peripherals@google.com",
                },
                Attr:         []string{"group:mainline", "informational"},
                SoftwareDeps: []string{"chrome"},
        })
}

// LaunchPrintmanagementFromLauncher verifies launching the print management app from the launcher.
func LaunchPrintmanagementFromLauncher(ctx context.Context, s *testing.State) {
        cr, err := chrome.New(ctx)
        if err != nil {
                s.Fatal("Failed to start Chrome: ", err)
        }

        cleanupCtx := ctx
        ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
        defer cancel()

        tconn, err := cr.TestAPIConn(ctx)
        if err != nil {
                s.Fatal("Failed to connect Test API: ", err)
        }
        defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

        if err := launcher.OpenLauncher(ctx, tconn); err != nil {
                s.Fatal("Failed to open launcher: ", err)
        }

        // Search for "Print Jobs".
        if err := launcher.Search(ctx, tconn, "Print Jobs"); err != nil {
                s.Fatal("Failed to search for Print Jobs: ", err)
        }

        // Print management app should be one of the search results.
        appNode, err := launcher.WaitForAppResult(ctx, tconn, apps.PrintManagement.Name, 15*time.Second)
        if err != nil {
                s.Fatal("Print management app does not exist in search result: ", err)
        }
        defer appNode.Release(ctx)

        // Clicking that result should open the print management app.
        if err := appNode.LeftClick(ctx); err != nil {
                s.Fatal("Failed to launch app from search result: ", err)
        }

        // App should be launched.
        if err := printmanagementapp.WaitForApp(ctx, tconn); err != nil {
                s.Fatal("Failed to launch print management app: ", err)
        }
}
