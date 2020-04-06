/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.accessibilitytest;

import android.app.Activity;
import android.content.Context;
import android.os.Bundle;

/** Test Activity containing EditText elements for arc.A11y* tast tests. */
public class EditTextActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.edittext_activity);
    }
}
