// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowAnimationJank,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Sample test to run ArcWindowAnimationJankTest.apk",
		Contacts:     []string{"yukashu@chromium.org", "sstan@chromium.org", "brpol@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"ArcWindowAnimationJankTest.apk", "config.pbtxt"},
		Timeout:      30 * time.Minute,
		Params: []testing.Param{{
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func WindowAnimationJank(ctx context.Context, s *testing.State) {

	const (
		apkName      = "ArcWindowAnimationJankTest.apk"
		pkgName      = "org.chromium.arc.testapp.windowanimationjank"
		activityName = "ElementLayoutActivity"
	)
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Installing: ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	a.Device().Root(ctx)
	if err := a.Device().Command(ctx, "shell", "echo 0 > /sys/kernel/debug/tracing/tracing_on").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to force enable trace: ", err)
	}
	traceConfigPath := s.DataPath("config.pbtxt")
	if err := a.PushFile(ctx, traceConfigPath, "/data/misc/perfetto-traces/"); err != nil {
		s.Error("Failed to push: ", err)
	}

	s.Log("Running test")
	if err := a.Command(ctx, "perfetto", "--txt", "--config", "/data/misc/perfetto-traces/config.pbtxt", "-o", "/data/misc/perfetto-traces/trace_file.perfetto-trace").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed: ", err)
	}

	s.Log("Starting app")

	act, err := arc.NewActivity(a, pkgName, "."+activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start the ElementLayoutActivity: ", err)
	}

	if err := a.PullFile(ctx, "/data/misc/perfetto-traces/trace_file.perfetto-trace", filepath.Join(s.OutDir(), "pulledtrace")); err != nil {
		s.Error("Failed to pull: ", err)
	}

}
