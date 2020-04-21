/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.quartersizedwindowzoomingtest;

import android.app.Activity;
import android.graphics.Color;
import android.os.Bundle;
import android.view.Window;

public class MainActivity extends Activity {
    StripeView stripeView;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Hide action bar
        requestWindowFeature(Window.FEATURE_NO_TITLE);

        stripeView = new StripeView(this);
        stripeView.setBackgroundColor(Color.WHITE);
        setContentView(stripeView);
    }
}
