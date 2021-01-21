/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.perappdensitytest;

import android.app.Activity;
import android.os.Bundle;
import android.view.Window;

/** A Second Activity for arc.PerAppDensity test. */
public class SecondActivity extends Activity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Hide action bar.
        requestWindowFeature(Window.FEATURE_NO_TITLE);
        setContentView(R.layout.second_activity);
    }
}
