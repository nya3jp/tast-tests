/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.dragdrop;

import android.content.ClipData;
import android.app.Activity;
import android.os.Bundle;
import android.widget.TextView;
import android.view.DragEvent;

public class DragDropActivity extends Activity {
    private TextView mDroppedDataView;
    private TextView mDroppedArea;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_dragdrop);

        mDroppedDataView = (TextView) findViewById(R.id.dropped_data_view);
        mDroppedArea = (TextView) findViewById(R.id.dropped_area);

        mDroppedArea.setOnDragListener((view, event) -> {
            switch (event.getAction()) {
                case DragEvent.ACTION_DRAG_STARTED:
                    return true;
                case DragEvent.ACTION_DROP:
                    mDroppedDataView.setText(event.getClipData().toString());
                    return true;
            }
            return false;
        });
    }
}
