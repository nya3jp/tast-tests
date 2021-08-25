/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedmousetest;

import android.app.Activity;
import android.os.Bundle;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

public class MainActivity extends Activity {

    private LinearLayout mLayoutMain;
    private Button mBtnLeftClick;
    private int mBtnLeftClickCounter = 1;

    private Button mBtnRightClick;
    private int mBtnRightClickCounter = 1;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        mLayoutMain = findViewById(R.id.layoutMain);

        // Add the text 'Mouse Left Click' when the left click button is pressed.
        // Always add the click counter so the tast test can make sure a single click
        // doesn't fire two events.
        mBtnLeftClick = findViewById(R.id.btnLeftClick);
        mBtnLeftClick.setOnClickListener(
                (v) -> {
                    TextView el = new TextView(this);
                    el.setText(String.format("MOUSE LEFT CLICK (%d)", mBtnLeftClickCounter));
                    mBtnLeftClickCounter++;
                    mLayoutMain.addView(el);
                });

        // Add the text 'Mouse Right Click' when the right click button is pressed.
        // Always add the click counter so the tast test can make sure a single click
        // doesn't fire two events.
        // 'OnContextClick' is fired natively when the user right clicks.
        mBtnRightClick = findViewById(R.id.btnRightClick);
        mBtnRightClick.setOnContextClickListener(
                (v) -> {
                    TextView el = new TextView(this);
                    el.setText(String.format("MOUSE RIGHT CLICK (%d)", mBtnRightClickCounter));
                    mBtnRightClickCounter++;
                    mLayoutMain.addView(el);
                    return true;
                });
    }
}
