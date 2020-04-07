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
    private TextView mIsScrollingText;

    boolean mIsScrolling = false;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        mNumPointers = findViewById(R.id.num_pointers);
        mInputSource = findViewById(R.id.input_source);
        mIsScrollingText = findViewById(R.id.is_scrolling);
    }

    @Override
    public boolean dispatchTouchEvent(MotionEvent ev) {
        if (!mIsScrolling && ev.getAction() == MotionEvent.ACTION_DOWN &&
                ev.getButtonState() == 0) {
            mIsScrolling = true;
        } if (mIsScrolling && ev.getAction() == MotionEvent.ACTION_UP ||
                ev.getAction() == MotionEvent.ACTION_CANCEL) {
            mIsScrolling = false;
        }
        mNumPointers.setText(Integer.toString(ev.getPointerCount()));
        mInputSource.setText(Integer.toString(ev.getSource()));
        mIsScrollingText.setText(Boolean.toString(mIsScrolling));
        return true;
    }

    @Override
    public boolean dispatchGenericMotionEvent(MotionEvent ev) {
        return dispatchTouchEvent(ev);
    }
}
