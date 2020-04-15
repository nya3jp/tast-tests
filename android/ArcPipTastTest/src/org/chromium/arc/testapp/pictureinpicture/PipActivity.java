/*
 * Copyright (C) 2019 The Android Open Source Project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
