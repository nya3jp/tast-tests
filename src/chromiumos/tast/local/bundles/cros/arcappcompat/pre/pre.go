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
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBooted = arc.NewPrecondition("arcappcompat_booted", appcompatGaia, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--disable-features=ArcResizeLock")...)

// AppCompatBootedInTabletMode returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedInTabletMode = arc.NewPrecondition("arcappcompat_booted_in_tablet_mode", appcompatGaia, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--disable-features=ArcResizeLock")...)

// AppCompatBootedForHearthstone is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Hearthstone credentials, and opt-in to the Play Store.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedForHearthstone = arc.NewPrecondition("arcappcompat_bootedForHearthstone", appcompatHearthstone, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--disable-features=ArcResizeLock")...)

// AppCompatBootedInTabletModeForHearthstone returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedInTabletModeForHearthstone = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForHearthstone", appcompatHearthstone, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--disable-features=ArcResizeLock")...)

// AppCompatBootedForNoteshelf is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Noteshelf credentials, and opt-in to the Play Store.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedForNoteshelf = arc.NewPrecondition("arcappcompat_bootedForNoteshelf", appcompatNoteshelf, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--disable-features=ArcResizeLock")...)

// AppCompatBootedInTabletModeForNoteshelf returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedInTabletModeForNoteshelf = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForNoteshelf", appcompatNoteshelf, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--disable-features=ArcResizeLock")...)

// AppCompatBootedForPhotolemur is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Photolemur credentials, and opt-in to the Play Store.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedForPhotolemur = arc.NewPrecondition("arcappcompat_bootedForPhotolemur", appcompatPhotolemur, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--disable-features=ArcResizeLock")...)

// AppCompatBootedInTabletModeForPhotolemur returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedInTabletModeForPhotolemur = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForPhotolemur", appcompatPhotolemur, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--disable-features=ArcResizeLock")...)

// AppCompatBootedForMyscriptNebo is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the MyscriptNebo credentials, and opt-in to the Play Store.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedForMyscriptNebo = arc.NewPrecondition("arcappcompat_bootedForMyscriptNebo", appcompatMyscriptNebo, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--disable-features=ArcResizeLock")...)

// AppCompatBootedInTabletModeForMyscriptNebo returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedInTabletModeForMyscriptNebo = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForMyscriptNebo", appcompatMyscriptNebo, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--disable-features=ArcResizeLock")...)

// AppCompatBootedForArtrage is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Artrage credentials, and opt-in to the Play Store.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedForArtrage = arc.NewPrecondition("arcappcompat_bootedForArtrage", appcompatArtrage, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--disable-features=ArcResizeLock")...)

// AppCompatBootedInTabletModeForArtrage returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedInTabletModeForArtrage = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForArtrage", appcompatArtrage, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--disable-features=ArcResizeLock")...)

// AppCompatBootedForCrossDJ is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the CrossDJ credentials, and opt-in to the Play Store.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedForCrossDJ = arc.NewPrecondition("arcappcompat_bootedForCrossDJ", appcompatCrossDJ, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--disable-features=ArcResizeLock")...)

// AppCompatBootedInTabletModeForCrossDJ returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
// TODO(b/193713610): Remove disable ArcResizeLock if a proper solution is found to handle ArcReziseLock.
var AppCompatBootedInTabletModeForCrossDJ = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForCrossDJ", appcompatCrossDJ, false /* O_DIRECT */, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--disable-features=ArcResizeLock")...)
