/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.keyboard;

import android.app.Activity;
import android.os.Bundle;
import android.view.View;
import android.widget.Button;

public class FocusChangeTestActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.focus_change_test_activity);

        final Button focus_switch_button = findViewById(
                R.id.focus_switch_button);
        focus_switch_button.setOnClickListener(v -> {
            final View text1 = findViewById(R.id.text1);
            if (!text1.isFocused()) {
                text1.requestFocus();
                return;
            }

            final View text2 = findViewById(R.id.text2);
            if (!text2.isFocused()) {
                text2.requestFocus();
                return;
            }
        });
    }
}
