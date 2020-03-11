/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputcompat;

import android.app.Activity;
import android.os.Bundle;
import android.view.MotionEvent;
import android.widget.TextView;

public class MainActivity extends Activity {
    private TextView mNumPointers;
    private TextView mInputSource;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        mNumPointers = findViewById(R.id.num_pointers);
        mInputSource = findViewById(R.id.input_source);
    }

    @Override
    public boolean dispatchGenericMotionEvent(MotionEvent ev) {
        mNumPointers.setText(Integer.toString(ev.getPointerCount()));
        mInputSource.setText(Integer.toString(ev.getSource()));
        return true;
    }
}
