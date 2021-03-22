/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.dragsource;

import android.content.ClipData;
import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;
import android.view.MotionEvent;
import android.view.View;

public class DragSourceActivity extends Activity {
    private TextView mDragArea;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_dragsource);

        mDragArea = (TextView) findViewById(R.id.drag_area);
        mDragArea.setOnTouchListener((View v, MotionEvent e) -> {
            if (e.getAction() != MotionEvent.ACTION_DOWN) {
                return false;
            }
            ClipData clipData = ClipData.newPlainText("", "hello world");
            return v.startDragAndDrop(
                    clipData, new View.DragShadowBuilder((mDragArea)), null, View.DRAG_FLAG_GLOBAL);
        });
    }
}
