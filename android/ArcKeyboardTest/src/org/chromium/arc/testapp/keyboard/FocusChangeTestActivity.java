/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.keyboard;

import android.app.Activity;
import android.os.Bundle;
import android.view.View;
import android.view.inputmethod.InputMethodManager;
import android.widget.Button;

public class FocusChangeTestActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.focus_change_test_activity);

        final Button focus_switch_button = findViewById(R.id.focus_switch_button);
        focus_switch_button.setOnClickListener(
                v -> {
                    toggleFocus();
                });

        final Button hide_button = findViewById(R.id.hide_button);
        hide_button.setOnClickListener(
                v -> {
                    hideKeyboard(v);
                });

        final Button hide_and_focus_switch_button = findViewById(R.id.hide_and_focus_switch_button);
        hide_and_focus_switch_button.setOnClickListener(
                v -> {
                    hideKeyboard(v);
                    toggleFocus();
                });
    }

    private void hideKeyboard(View v) {
        final InputMethodManager imm = getSystemService(InputMethodManager.class);
        imm.hideSoftInputFromWindow(v.getWindowToken(), 0);
    }

    private void toggleFocus() {
        final View text1 = findViewById(R.id.text1);
        if (!text1.isFocused()) {
            text1.requestFocus();
            return;
        }

        final View text2 = findViewById(R.id.text2);
        text2.requestFocus();
    }
}
