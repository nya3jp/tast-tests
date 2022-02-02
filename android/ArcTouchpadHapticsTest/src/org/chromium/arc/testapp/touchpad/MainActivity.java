/*
 * Copyright 2022 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.touchpad;

import android.app.Activity;
import android.os.Bundle;
import android.view.HapticFeedbackConstants;
import android.widget.Button;

public class MainActivity extends Activity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        final Button vibrate = findViewById(R.id.vibrate);
        vibrate.setOnClickListener((v) -> {
            v.performHapticFeedback(HapticFeedbackConstants.VIRTUAL_KEY);
        });
    }
}
