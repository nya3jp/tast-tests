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
    private TextView mLastPreImeKey;
    private TextView mLastKeyDown;
    private boolean mConsumeEvents = false;

    public CaptureKeyPreImeView(Context context) {
        super(context);
    }

    public CaptureKeyPreImeView(Context context, AttributeSet attrs) {
        super(context, attrs);
    }

    public CaptureKeyPreImeView(Context context, AttributeSet attrs, int defStyleAttr) {
        super(context, attrs, defStyleAttr);
    }

    void setLastKeyEventLabels(TextView lastPreImeKey, TextView lastKeyDown) {
        mLastPreImeKey = lastPreImeKey;
        mLastPreImeKey.setText("null");
        mLastKeyDown = lastKeyDown;
        mLastKeyDown.setText("null");
    }

    void startConsumingEvents() {
        mConsumeEvents = true;
    }

    @Override
    public boolean onKeyPreIme(int keyCode, KeyEvent event) {
        if (event.getAction() == KeyEvent.ACTION_DOWN) {
            mLastPreImeKey.setText("key down: keyCode=" + keyCode);
        }
        if (mConsumeEvents) {
            return true;
        }
        return super.onKeyPreIme(keyCode, event);
    }

    @Override
    public boolean onKeyDown(int keyCode, KeyEvent event) {
        mLastKeyDown.setText("key down: keyCode=" + keyCode);
        return super.onKeyDown(keyCode, event);
    }
}
