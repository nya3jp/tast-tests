// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbprinter

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

// The module load timeout is arbitrarily set. We don't have a clear
// idea of how long it takes, but we suspect it is not more than this.
const moduleLoadTimeout = 10 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "virtualUsbPrinterModulesLoaded",
		Desc:            "Kernel modules necessary for `virtual-usb-printer` loaded",
		Contacts:        []string{"cros-printing-dev@chromium.org"},
		Impl:            &loadModuleFixture{},
		SetUpTimeout:    moduleLoadTimeout,
		TearDownTimeout: moduleLoadTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name:            "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
		Desc:            "Kernel modules necessary for `virtual-usb-printer` loaded (with `chromeLoggedIn` fixture)",
		Contacts:        []string{"cros-printing-dev@chromium.org"},
		Impl:            &loadModuleFixture{},
		Parent:          "chromeLoggedIn",
		SetUpTimeout:    moduleLoadTimeout,
		TearDownTimeout: moduleLoadTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name:            "virtualUsbPrinterModulesLoadedWithLacros",
		Desc:            "Kernel modules necessary for `virtual-usb-printer` loaded (with `lacros` fixture)",
		Contacts:        []string{"cros-printing-dev@chromium.org"},
		Impl:            &loadModuleFixture{},
		Parent:          "lacros",
		SetUpTimeout:    moduleLoadTimeout,
		TearDownTimeout: moduleLoadTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name:            "virtualUsbPrinterModulesLoadedWithArcBooted",
		Desc:            "Kernel modules necessary for `virtual-usb-printer` loaded (with `arcBooted` fixture)",
		Contacts:        []string{"cros-printing-dev@chromium.org"},
		Impl:            &loadModuleFixture{},
		Parent:          "arcBooted",
		SetUpTimeout:    moduleLoadTimeout,
		TearDownTimeout: moduleLoadTimeout,
	})
}

// Implements `testing.FixtureImpl` without any members.
type loadModuleFixture struct {
}

// Loads the required kernel modules. Poisons all dependent tests on
// failure.
func (*loadModuleFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cmd := testexec.CommandContext(ctx, "modprobe", "-a", "usbip_core", "vhci-hcd")
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to install usbip kernel modules: ", err)
	}
	// Provides pass-through for the value yielded by the parent fixture.
	return s.ParentValue()
}

// Does nothing on the assumption that we have no need to reload the
// kernel modules between tests.
func (*loadModuleFixture) Reset(ctx context.Context) error {
	return nil
}

func (*loadModuleFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (*loadModuleFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

// Unloads the required kernel modules.
func (*loadModuleFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	cmd := testexec.CommandContext(ctx, "modprobe", "-r", "-a", "vhci-hcd", "usbip_core")
	if err := cmd.Run(); err != nil {
		s.Error("Failed to remove usbip kernel modules: ", err)
	}
}
