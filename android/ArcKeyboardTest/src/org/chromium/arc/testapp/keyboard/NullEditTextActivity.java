/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.keyboard;

import android.app.Activity;
import android.os.Bundle;
import android.text.InputType;
import android.widget.EditText;

public class NullEditTextActivity extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);
        EditText text = (EditText) findViewById(R.id.text);
        text.setInputType(InputType.TYPE_NULL);
    }
}
