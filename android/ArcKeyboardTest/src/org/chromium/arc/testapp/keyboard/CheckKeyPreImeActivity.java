/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.keyboard;

import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;

public class CheckKeyPreImeActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.check_key_pre_ime_activity);

        final CaptureKeyPreImeView field = (CaptureKeyPreImeView) findViewById(R.id.text);
        final TextView lastKeyDown = (TextView) findViewById(R.id.last_key_down);
        final TextView lastKeyUp = (TextView) findViewById(R.id.last_key_up);
        field.setLastKeyEventLabels(lastKeyDown, lastKeyUp);
    }
}
