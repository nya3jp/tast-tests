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
<<<<<<< HEAD   (0e2500 [M93] Skip PlatformEncoding tests on grunt)
var AppCompatBooted = arc.NewPrecondition("arcappcompat_booted", appcompatGaia, arc.DisableSyncFlags()...)
=======
var AppCompatBooted = arc.NewPrecondition("arcappcompat_booted", appcompatGaia, append(arc.DisableSyncFlags())...)
>>>>>>> CHANGE (030896 arcappcompat: Automate resize lock testcase for arcappcompat)

// AppCompatBootedInTabletMode returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletMode = arc.NewPrecondition("arcappcompat_booted_in_tablet_mode", appcompatGaia, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForHearthstone is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Hearthstone credentials, and opt-in to the Play Store.
<<<<<<< HEAD   (0e2500 [M93] Skip PlatformEncoding tests on grunt)
var AppCompatBootedForHearthstone = arc.NewPrecondition("arcappcompat_bootedForHearthstone", appcompatHearthstone, arc.DisableSyncFlags()...)
=======
var AppCompatBootedForHearthstone = arc.NewPrecondition("arcappcompat_bootedForHearthstone", appcompatHearthstone, append(arc.DisableSyncFlags())...)
>>>>>>> CHANGE (030896 arcappcompat: Automate resize lock testcase for arcappcompat)

// AppCompatBootedInTabletModeForHearthstone returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForHearthstone = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForHearthstone", appcompatHearthstone, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForNoteshelf is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Noteshelf credentials, and opt-in to the Play Store.
<<<<<<< HEAD   (0e2500 [M93] Skip PlatformEncoding tests on grunt)
var AppCompatBootedForNoteshelf = arc.NewPrecondition("arcappcompat_bootedForNoteshelf", appcompatNoteshelf, arc.DisableSyncFlags()...)
=======
var AppCompatBootedForNoteshelf = arc.NewPrecondition("arcappcompat_bootedForNoteshelf", appcompatNoteshelf, append(arc.DisableSyncFlags())...)
>>>>>>> CHANGE (030896 arcappcompat: Automate resize lock testcase for arcappcompat)

// AppCompatBootedInTabletModeForNoteshelf returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForNoteshelf = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForNoteshelf", appcompatNoteshelf, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForPhotolemur is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Photolemur credentials, and opt-in to the Play Store.
<<<<<<< HEAD   (0e2500 [M93] Skip PlatformEncoding tests on grunt)
var AppCompatBootedForPhotolemur = arc.NewPrecondition("arcappcompat_bootedForPhotolemur", appcompatPhotolemur, arc.DisableSyncFlags()...)
=======
var AppCompatBootedForPhotolemur = arc.NewPrecondition("arcappcompat_bootedForPhotolemur", appcompatPhotolemur, append(arc.DisableSyncFlags())...)
>>>>>>> CHANGE (030896 arcappcompat: Automate resize lock testcase for arcappcompat)

// AppCompatBootedInTabletModeForPhotolemur returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForPhotolemur = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForPhotolemur", appcompatPhotolemur, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForMyscriptNebo is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the MyscriptNebo credentials, and opt-in to the Play Store.
<<<<<<< HEAD   (0e2500 [M93] Skip PlatformEncoding tests on grunt)
var AppCompatBootedForMyscriptNebo = arc.NewPrecondition("arcappcompat_bootedForMyscriptNebo", appcompatMyscriptNebo, arc.DisableSyncFlags()...)
=======
var AppCompatBootedForMyscriptNebo = arc.NewPrecondition("arcappcompat_bootedForMyscriptNebo", appcompatMyscriptNebo, append(arc.DisableSyncFlags())...)
>>>>>>> CHANGE (030896 arcappcompat: Automate resize lock testcase for arcappcompat)

// AppCompatBootedInTabletModeForMyscriptNebo returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForMyscriptNebo = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForMyscriptNebo", appcompatMyscriptNebo, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForArtrage is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the Artrage credentials, and opt-in to the Play Store.
<<<<<<< HEAD   (0e2500 [M93] Skip PlatformEncoding tests on grunt)
var AppCompatBootedForArtrage = arc.NewPrecondition("arcappcompat_bootedForArtrage", appcompatArtrage, arc.DisableSyncFlags()...)
=======
var AppCompatBootedForArtrage = arc.NewPrecondition("arcappcompat_bootedForArtrage", appcompatArtrage, append(arc.DisableSyncFlags())...)
>>>>>>> CHANGE (030896 arcappcompat: Automate resize lock testcase for arcappcompat)

// AppCompatBootedInTabletModeForArtrage returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForArtrage = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForArtrage", appcompatArtrage, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)

// AppCompatBootedForCrossDJ is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the CrossDJ credentials, and opt-in to the Play Store.
<<<<<<< HEAD   (0e2500 [M93] Skip PlatformEncoding tests on grunt)
var AppCompatBootedForCrossDJ = arc.NewPrecondition("arcappcompat_bootedForCrossDJ", appcompatCrossDJ, arc.DisableSyncFlags()...)
=======
var AppCompatBootedForCrossDJ = arc.NewPrecondition("arcappcompat_bootedForCrossDJ", appcompatCrossDJ, append(arc.DisableSyncFlags())...)
>>>>>>> CHANGE (030896 arcappcompat: Automate resize lock testcase for arcappcompat)

// AppCompatBootedInTabletModeForCrossDJ returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
var AppCompatBootedInTabletModeForCrossDJ = arc.NewPrecondition("arcappcompat_booted_in_tablet_modeForCrossDJ", appcompatCrossDJ, append(arc.DisableSyncFlags(), "--force-tablet-mode=touch_view", "--enable-virtual-keyboard")...)
