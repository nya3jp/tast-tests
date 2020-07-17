/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.pictureinpicture;

import android.app.Activity;
import android.app.PictureInPictureParams;
import android.os.Bundle;
import android.util.Rational;
import android.widget.Button;

/** Test Activity for the PIP Tast Test. */
public class PipActivity extends Activity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        setContentView(R.layout.pip_activity);

        Button enter_pip = findViewById(R.id.enter_pip_button);
        enter_pip.setOnClickListener(
                view -> {
                    PictureInPictureParams params =
                            new PictureInPictureParams.Builder()
                                    .setAspectRatio(new Rational(1, 1))
                                    .build();
                    enterPictureInPictureMode(params);
                });
    }

    @Override
    protected void onUserLeaveHint() {
        super.onUserLeaveHint();

        PictureInPictureParams params =
                new PictureInPictureParams.Builder().setAspectRatio(new Rational(1, 1)).build();
        enterPictureInPictureMode(params);
    }
}
