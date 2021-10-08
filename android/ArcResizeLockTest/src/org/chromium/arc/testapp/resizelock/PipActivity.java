/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.resizelock;

import android.app.Activity;
import android.app.PictureInPictureParams;
import android.os.Bundle;
import android.util.Rational;
import android.widget.Button;

public class PipActivity extends Activity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        setContentView(R.layout.main_activity);
    }

    @Override
    protected void onUserLeaveHint() {
        super.onUserLeaveHint();

        PictureInPictureParams params =
                new PictureInPictureParams.Builder().setAspectRatio(new Rational(1, 1)).build();
        enterPictureInPictureMode(params);
    }
}