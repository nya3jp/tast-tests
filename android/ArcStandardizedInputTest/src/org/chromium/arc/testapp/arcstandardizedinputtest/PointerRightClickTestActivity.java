/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.arcstandardizedinputtest;

import android.app.Activity;
import android.os.Bundle;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

public class PointerRightClickTestActivity extends Activity {

    private LinearLayout mLayoutMain;
    private int mBtnRightClickCounter = 1;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_pointer_right_click_test);

        mLayoutMain = findViewById(R.id.layoutMain);

        // Add the text 'Pointer Right Click' when the right click button is pressed.
        // Always add the click counter so the tast test can make sure a single click
        // doesn't fire two events.
        // 'OnContextClick' is fired natively when the user right clicks.
        Button btnRightClick = findViewById(R.id.btnRightClick);
        btnRightClick.setOnContextClickListener(
                (v) -> {
                    TextView el = new TextView(this);
                    el.setText(String.format("POINTER RIGHT CLICK (%d)", mBtnRightClickCounter));
                    mBtnRightClickCounter++;
                    mLayoutMain.addView(el);
                    return true;
                });
    }
}
