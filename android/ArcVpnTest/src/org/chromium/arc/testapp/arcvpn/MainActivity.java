/*
 * Copyright 2022 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcvpn;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;

/**
 * Test app that starts a simple VPN. It's not expected to actually forward data in/out, just to
 * register some VPN with the system.
 *
 * To preauthorize the package and bypass user dialog:
 *   $ adb shell dumpsys wifi authorize-vpn org.chromium.arc.testapp.arcvpn
 *
 * To start the activity which then starts the service:
 *   $ adb shell am start \
 *       org.chromium.arc.testapp.arcvpn/org.chromium.arc.testapp.arcvpn.MainActivity
 */
public class MainActivity extends Activity {

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        startService(new Intent(this, ArcTestVpnService.class));
    }
}
