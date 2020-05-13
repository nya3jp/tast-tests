/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.windowmanager;

import android.app.ListActivity;
import android.content.Context;
import android.graphics.Point;
import android.hardware.display.DisplayManager;
import android.os.Bundle;
import android.util.Log;
import android.view.Display;
import android.widget.SimpleAdapter;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

public class DisplayTestActivity extends ListActivity implements DisplayManager.DisplayListener {
    private static final String TAG = "ArcWMTestApp";

    private static final String MULTIPLICATION_SIGN = "\u00d7";

    private static final String[] COLUMNS =
            new String[] {"display_id", "display_name", "display_size", "display_real_size"};

    private static final int[] VIEWS = new int[] {
            R.id.display_id, R.id.display_name, R.id.display_size, R.id.display_real_size};

    private DisplayManager mDisplayManager;
    private SimpleAdapter mAdapter;

    private final List<Map<String, Object>> mData = new ArrayList<>();

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        mDisplayManager = (DisplayManager) getSystemService(Context.DISPLAY_SERVICE);

        mAdapter = new SimpleAdapter(this, mData, R.layout.display_list_item, COLUMNS, VIEWS);
        setListAdapter(mAdapter);
    }

    @Override
    public void onStart() {
        super.onStart();
        mDisplayManager.registerDisplayListener(this, null);
        refresh();
    }

    @Override
    public void onStop() {
        super.onStop();
        mDisplayManager.unregisterDisplayListener(this);
    }

    @Override
    public void onDisplayAdded(int displayId) {
        Log.i(TAG, "Added display " + displayId);
        refresh();
    }

    @Override
    public void onDisplayChanged(int displayId) {
        Log.i(TAG, "Changed display " + displayId);
        refresh();
    }

    @Override
    public void onDisplayRemoved(int displayId) {
        Log.i(TAG, "Removed display " + displayId);
        refresh();
    }

    private void refresh() {
        mData.clear();

        for (Display display : mDisplayManager.getDisplays()) {
            Map<String, Object> item = new HashMap<>();
            item.put(COLUMNS[0], display.getDisplayId());
            item.put(COLUMNS[1], display.getName());

            Point size = new Point();
            display.getSize(size);
            item.put(COLUMNS[2], sizeToString(size));
            display.getRealSize(size);
            item.put(COLUMNS[3], sizeToString(size));

            mData.add(item);
        }

        mAdapter.notifyDataSetChanged();
    }

    private static String sizeToString(Point size) {
        return size.x + MULTIPLICATION_SIGN + size.y;
    }
}
