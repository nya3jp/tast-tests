// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dgapi2

import "time"

const (
	tryLimit              = 3
	uiTimeout             = 30 * time.Second
	appID                 = "hccligcafeiehpaolmeeialgndmkhojb"
	appName               = "Web Play Billing Sample App"
	pkgName               = "com.potatoh.playbilling"
	accountURL            = "https://accounts.google.com"
	targetURL             = "https://twa-sample-cros-sa.web.app/"
	appBarJS              = "document.getElementById('app-bar')"
	profileMenuJS         = appBarJS + ".shadowRoot.getElementById('profile-menu')"
	profileMenuLoggedInJS = profileMenuJS + ".loggedIn"
	profileMenuSignInJS   = profileMenuJS + "._requestSignIn()"
	profileMenuSignOutJS  = profileMenuJS + "._requestSignOut()"
	logBoxJS              = "document.getElementById('log-box')"
	logBoxLogLinesJS      = logBoxJS + `.innerHTML.replace(/<br>\s*$/, '').split(/\s{4,}/)`
	itemListJS            = "document.getElementById('items-to-buy')"
	itemsJS               = itemListJS + ".shadowRoot.querySelectorAll('sku-holder')"
	oneTimePurchaseType   = "onetime"
)
