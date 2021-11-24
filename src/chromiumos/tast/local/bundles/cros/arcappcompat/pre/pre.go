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

var appcompatGaiaLoginPool = &arc.GaiaLoginPoolVars{
	PoolVar: "arcappcompat.gaiaPoolDefault",
}

var appcompatHearthstone = &arc.GaiaVars{
	UserVar: "arcappcompat.Hearthstone.username",
	PassVar: "arcappcompat.Hearthstone.password",
}

var appcompatNoteshelf = &arc.GaiaVars{
	UserVar: "arcappcompat.Noteshelf.username",
	PassVar: "arcappcompat.Noteshelf.password",
}

var appcompatPhotolemur = &arc.GaiaVars{
	UserVar: "arcappcompat.Photolemur.username",
	PassVar: "arcappcompat.Photolemur.password",
}

var appcompatMyscriptNebo = &arc.GaiaVars{
	UserVar: "arcappcompat.MyscriptNebo.username",
	PassVar: "arcappcompat.MyscriptNebo.password",
}

var appcompatArtrage = &arc.GaiaVars{
	UserVar: "arcappcompat.Artrage.username",
	PassVar: "arcappcompat.Artrage.password",
}

var appcompatCrossDJ = &arc.GaiaVars{
	UserVar: "arcappcompat.CrossDJ.username",
	PassVar: "arcappcompat.CrossDJ.password",
}

// AppCompatBooted is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the app compat credentials, and opt-in to the Play Store.
var AppCompatBooted = arc.NewPrecondition("arcappcompat_booted", appcompatGaia, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags())...)

// AppCompatBootedInTabletMode returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletMode = arc.NewPrecondition("arcappcompat_booted_in_tablet_mode", appcompatGaia, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForHearthstone is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Hearthstone credentials, and opt-in to the Play Store.
var AppCompatBootedForHearthstone = arc.NewPrecondition("arcappcompat_bootedForHearthstone", appcompatHearthstone, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags())...)

// AppCompatBootedInTabletModeForHearthstone returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForHearthstone = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForHearthstone", appcompatHearthstone, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForNoteshelf is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Noteshelf credentials, and opt-in to the Play Store.
var AppCompatBootedForNoteshelf = arc.NewPrecondition("arcappcompat_bootedForNoteshelf", appcompatNoteshelf, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags())...)

// AppCompatBootedInTabletModeForNoteshelf returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForNoteshelf = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForNoteshelf", appcompatNoteshelf, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForPhotolemur is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Photolemur credentials, and opt-in to the Play Store.
var AppCompatBootedForPhotolemur = arc.NewPrecondition("arcappcompat_bootedForPhotolemur", appcompatPhotolemur, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags())...)

// AppCompatBootedInTabletModeForPhotolemur returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForPhotolemur = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForPhotolemur", appcompatPhotolemur, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForMyscriptNebo is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the MyscriptNebo credentials, and opt-in to the Play Store.
var AppCompatBootedForMyscriptNebo = arc.NewPrecondition("arcappcompat_bootedForMyscriptNebo", appcompatMyscriptNebo, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags())...)

// AppCompatBootedInTabletModeForMyscriptNebo returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForMyscriptNebo = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForMyscriptNebo", appcompatMyscriptNebo, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForArtrage is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Artrage credentials, and opt-in to the Play Store.
var AppCompatBootedForArtrage = arc.NewPrecondition("arcappcompat_bootedForArtrage", appcompatArtrage, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags())...)

// AppCompatBootedInTabletModeForArtrage returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForArtrage = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForArtrage", appcompatArtrage, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForCrossDJ is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the CrossDJ credentials, and opt-in to the Play Store.
var AppCompatBootedForCrossDJ = arc.NewPrecondition("arcappcompat_bootedForCrossDJ", appcompatCrossDJ, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags())...)

// AppCompatBootedInTabletModeForCrossDJ returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForCrossDJ = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForCrossDJ", appcompatCrossDJ, nil /* GAIALOGINPOOLVARS */, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedUsingTestAccountPool is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the app compat credentials, and opt-in to the Play Store.
var AppCompatBootedUsingTestAccountPool = arc.NewPrecondition("arcappcompat_booted_testpool", nil, appcompatGaiaLoginPool, false /* O_DIRECT */, append(arc.DisableSyncFlags())...)

// AppCompatBootedInTabletModeUsingTestAccountPool returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeUsingTestAccountPool = arc.NewPrecondition("arcappcompat_booted_in_tablet_mode_testpool", nil, appcompatGaiaLoginPool, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)
