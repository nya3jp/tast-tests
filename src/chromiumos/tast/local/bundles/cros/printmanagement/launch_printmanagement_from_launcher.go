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
                Pre:          chrome.LoggedIn(),
        })
}

// LaunchPrintmanagementFromLauncher verifies launching the print management app from the launcher.
func LaunchPrintmanagementFromLauncher(ctx context.Context, s *testing.State) {
        cr := s.PreValue().(*chrome.Chrome)

        cleanupCtx := ctx
        ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
        defer cancel()

        tconn, err := cr.TestAPIConn(ctx)
        if err != nil {
                s.Fatal("Failed to connect Test API: ", err)
        }
        defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

        if err := launcher.SearchAndLaunch(ctx, tconn, apps.PrintManagement.Name); err != nil {
                s.Fatal("Failed to open launcher and search for Print Jobs: ", err)
        }

        // App should be launched.
        if err := printmanagementapp.WaitForApp(ctx, tconn); err != nil {
                s.Fatal("Failed to launch print management app: ", err)
        }
}
