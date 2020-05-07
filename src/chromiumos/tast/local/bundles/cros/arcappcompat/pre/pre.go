// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre contains the preconditions used by the arcappcompat tests.
package pre

import (
	"chromiumos/tast/local/arc"
)

var appcompatGaia = &arc.GaiaVars{
	UserVar: "arcappcompat.username",
	PassVar: "arcappcompat.password",
}

var appcompatHearthstone = &arc.GaiaVars{
	"arcappcompat.Hearthstone.username",
	"arcappcompat.Hearthstone.password",
}

// AppCompatBooted is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the app compat credentials, and opt-in to the Play Store.
var AppCompatBooted = arc.NewPrecondition("arcappcompat_booted", false, appcompatGaia, "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")

// AppCompatVMBooted is a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that ARC VM, and not the ARC Container, is enabled in this precondition.
var AppCompatVMBooted = arc.NewPrecondition("arcappcompat_vmbooted", true, appcompatGaia, "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")

// AppCompatBootedInTabletMode returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletMode = arc.NewPrecondition("arcappcompat_booted_in_tablet_mode", false, appcompatGaia, "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")

// AppCompatVMBootedInTabletMode returns a precondition similar to BootedInTabletModeAppCompat(). The only difference from BootedInTabletModeAppCompat() is
// that ARC VM, and not the ARC Container, is enabled in this precondition.
var AppCompatVMBootedInTabletMode = arc.NewPrecondition("arcappcompat_vmbooted_in_tablet_mode", true, appcompatGaia, "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")

// AppCompatBootedForHearthstone is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Hearthstone credentials, and opt-in to the Play Store.
var AppCompatBootedForHearthstone = arc.NewPrecondition("arcappcompat_booted", false, appcompatHearthstone, "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")

// AppCompatVMBootedForHearthstone is a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that ARC VM, and not the ARC Container, is enabled in this precondition.
var AppCompatVMBootedForHearthstone = arc.NewPrecondition("arcappcompat_vmbooted", true, appcompatHearthstone, "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")

// AppCompatBootedInTabletModeForHearthstone returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForHearthstone = arc.NewPrecondition("arcappcompat_booted_in_tablet_mode", false, appcompatHearthstone, "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")

// AppCompatVMBootedInTabletModeForHearthstone returns a precondition similar to BootedInTabletModeAppCompat(). The only difference from BootedInTabletModeAppCompat() is
// that ARC VM, and not the ARC Container, is enabled in this precondition.
var AppCompatVMBootedInTabletModeForHearthstone = arc.NewPrecondition("arcappcompat_vmbooted_in_tablet_mode", true, appcompatHearthstone, "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off")
