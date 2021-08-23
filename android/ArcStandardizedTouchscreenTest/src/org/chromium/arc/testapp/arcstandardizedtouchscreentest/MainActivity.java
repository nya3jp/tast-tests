/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedtouchscreentest;

import android.app.Activity;
import android.os.Bundle;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

public class MainActivity extends Activity {

    private LinearLayout mLayoutMain;

    private Button mBtnTap;
    private int mBtnTapCounter = 1;

    private Button mBtnLongTap;
    private int mBtnLongTapCounter = 1;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        mLayoutMain = findViewById(R.id.layoutMain);

        // Add the text 'Touchscreen Tap' when the button is pressed.
        // Always add the counter so the tast test can make sure a single tap
        // doesn't fire two events.
        mBtnTap = findViewById(R.id.btnTap);
        mBtnTap.setOnClickListener(v -> {
            addTextViewToLayout(String.format("TOUCHSCREEN TAP (%d)", mBtnTapCounter));
            mBtnTapCounter++;
        });

        // Add the text 'Touchscreen Long Tap' when a long tap is performed. Use the same
        // counter logic as above to make sure multiple events aren't fired for a single event.
        mBtnLongTap = findViewById(R.id.btnLongTap);
        mBtnLongTap.setOnLongClickListener(v -> {
            addTextViewToLayout(String.format("TOUCHSCREEN LONG TAP (%d)", mBtnLongTapCounter));
            mBtnLongTapCounter++;
            return true;
        });
    }

    private void addTextViewToLayout(String text) {
        TextView el = new TextView(this);
        el.setText(text);
        mLayoutMain.addView(el);
    }
}
