/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.longpress;

import android.app.Activity;
import android.content.Context;
import android.os.Bundle;
import android.view.View;
import android.widget.TextView;

public class MainActivity extends Activity implements View.OnLongClickListener {
    private View mMainView;
    private TextView mLongPressCountView;
    private int mLongPressCount = 0;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        mMainView = findViewById(R.id.main_view);
        mLongPressCountView = findViewById(R.id.long_press_count);

        mMainView.setOnLongClickListener(this);
    }

    @Override
    public boolean onLongClick(View v) {
        mLongPressCount++;
        mLongPressCountView.setText(Integer.toString(mLongPressCount));
        return true;
    }
}
