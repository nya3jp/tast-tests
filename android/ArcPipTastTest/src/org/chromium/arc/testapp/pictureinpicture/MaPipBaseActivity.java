/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.pictureinpicture;

import android.app.Activity;
import android.os.Bundle;
import android.content.Intent;
import android.widget.Button;

/** Blank Activity for the PIP Tast Test. Used to trigger auto-PIP */
public class MaPipBaseActivity extends Activity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        setContentView(R.layout.mapip_base_activity);

        Button launch_pip_activity_button = findViewById(R.id.launch_pip_activity_button);
        launch_pip_activity_button.setOnClickListener(
                view -> {
                    Intent intent = new Intent(this, PipActivity.class);
                    startActivity(intent);
                });
    }
}
