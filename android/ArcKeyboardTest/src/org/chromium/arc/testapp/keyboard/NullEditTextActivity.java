/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.keyboard;

import android.app.Activity;
import android.content.Context;
import android.os.Bundle;
import android.text.InputType;
import android.widget.EditText;
import android.widget.TextView;
import android.view.KeyEvent;
import android.view.inputmethod.InputMethodManager;

public class NullEditTextActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        final EditText text = (EditText) findViewById(R.id.text);
        text.setInputType(InputType.TYPE_NULL);
        text.setOnClickListener(v -> {
            final InputMethodManager imm =
                    (InputMethodManager) getSystemService(Context.INPUT_METHOD_SERVICE);
            if (imm == null) {
                return;
            }
            imm.showSoftInput(v, 0);
            return;
        });

        final TextView lastKeyDown = (TextView) findViewById(R.id.last_key_down);
        final TextView lastKeyUp = (TextView) findViewById(R.id.last_key_up);
        text.setOnKeyListener((v, keyCode, event) -> {
            if (event.getAction() == KeyEvent.ACTION_DOWN) {
                lastKeyDown.setText("key down: keyCode=" + keyCode);
            } else if (event.getAction() == KeyEvent.ACTION_UP) {
                lastKeyUp.setText("key up: keyCode=" + keyCode);
            }
            return true;
        });
    }
}
