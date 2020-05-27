/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.keyboard;

import android.content.Context;
import android.util.AttributeSet;
import android.view.KeyEvent;
import android.widget.EditText;
import android.widget.TextView;

class CaptureKeyPreImeView extends EditText {
    private TextView mLastKeyDown;
    private TextView mLastKeyUp;

    public CaptureKeyPreImeView(Context context) {
        super(context);
    }

    public CaptureKeyPreImeView(Context context, AttributeSet attrs) {
        super(context, attrs);
    }

    public CaptureKeyPreImeView(Context context, AttributeSet attrs, int defStyleAttr) {
        super(context, attrs, defStyleAttr);
    }

    void setLastKeyEventLabels(TextView lastKeyDown, TextView lastKeyUp) {
        mLastKeyDown = lastKeyDown;
        mLastKeyUp = lastKeyUp;
    }

    @Override
    public boolean onKeyPreIme(int keyCode, KeyEvent event) {
        if (event.getAction() == KeyEvent.ACTION_DOWN) {
            mLastKeyDown.setText("key down: keyCode=" + keyCode);
        } else if (event.getAction() == KeyEvent.ACTION_UP) {
            mLastKeyUp.setText("key up: keyCode=" + keyCode);
        }
        return false;
    }
}
