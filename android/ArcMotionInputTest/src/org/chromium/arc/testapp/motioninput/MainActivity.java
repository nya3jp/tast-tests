/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.motioninput;

import android.util.Pair;
import java.util.ArrayList;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;
import android.app.Activity;
import android.os.Bundle;
import android.view.InputDevice;
import android.view.MotionEvent;
import android.widget.TextView;
import java.util.List;

public class MainActivity extends Activity {

    private static final String KEY_ACTION = "action";
    private static final String KEY_SOURCES = "sources";
    private static final String KEY_POINTER_AXES = "pointer_axes";
    private static final String KEY_DEVICE_ID = "device_id";

    private static final String SOURCE_KEYBOARD = "keyboard";
    private static final String SOURCE_DPAD = "dpad";
    private static final String SOURCE_TOUCHSCREEN = "touchscreen";
    private static final String SOURCE_MOUSE = "mouse";
    private static final String SOURCE_STYLUS = "stylus";
    private static final String SOURCE_TRACKBALL = "trackball";
    private static final String SOURCE_MOUSE_RELATIVE = "mouse_relative";
    private static final String SOURCE_TOUCHPAD = "touchpad";
    private static final String SOURCE_JOYSTICK = "joystick";
    private static final String SOURCE_GAMEPAD = "gamepad";

    private static final List<Pair<Integer, String>> sSourcePairs = new ArrayList<>();
    static {
        sSourcePairs.add(new Pair<>(InputDevice.SOURCE_KEYBOARD, SOURCE_KEYBOARD));
        sSourcePairs.add(new Pair<>(InputDevice.SOURCE_DPAD, SOURCE_DPAD));
        sSourcePairs.add(new Pair<>(InputDevice.SOURCE_TOUCHSCREEN, SOURCE_TOUCHSCREEN));
        sSourcePairs.add(new Pair<>(InputDevice.SOURCE_MOUSE, SOURCE_MOUSE));
        sSourcePairs.add(new Pair<>(InputDevice.SOURCE_STYLUS, SOURCE_STYLUS));
        sSourcePairs.add(new Pair<>(InputDevice.SOURCE_TRACKBALL, SOURCE_TRACKBALL));
        sSourcePairs.add(new Pair<>(InputDevice.SOURCE_MOUSE_RELATIVE, SOURCE_MOUSE_RELATIVE));
        sSourcePairs.add(new Pair<>(InputDevice.SOURCE_TOUCHPAD, SOURCE_TOUCHPAD));
        sSourcePairs.add(new Pair<>(InputDevice.SOURCE_JOYSTICK, SOURCE_JOYSTICK));
        sSourcePairs.add(new Pair<>(InputDevice.SOURCE_GAMEPAD, SOURCE_GAMEPAD));
    }

    private TextView mTvMotionEvents;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main_activity);

        mTvMotionEvents = findViewById(R.id.motion_event);
    }

    @Override
    public boolean dispatchGenericMotionEvent(MotionEvent ev) {
        mTvMotionEvents.setText(getJSONStringFromMotionEvent(ev));
        return true;
    }

    @Override
    public boolean dispatchTouchEvent(MotionEvent ev) {
        mTvMotionEvents.setText(getJSONStringFromMotionEvent(ev));
        return true;
    }

    @Override
    public boolean dispatchTrackballEvent(MotionEvent ev) {
        mTvMotionEvents.setText(getJSONStringFromMotionEvent(ev));
        return true;
    }

    protected static String getJSONStringFromMotionEvent(MotionEvent event) {
        final InputDevice device = InputDevice.getDevice(event.getDeviceId());
        if (device == null) {
            return "";
        }
        final List<InputDevice.MotionRange> motionRanges = device.getMotionRanges();
        if (motionRanges == null) {
            return "";
        }

        try {
            final JSONObject eventObj = new JSONObject();
            eventObj.put(KEY_ACTION, MotionEvent.actionToString(event.getAction()));
            eventObj.put(KEY_DEVICE_ID, event.getDeviceId());
            final JSONArray sourcesArr = new JSONArray();
            getStringsForSource(event.getSource()).forEach(sourcesArr::put);
            eventObj.put(KEY_SOURCES, sourcesArr);
            final JSONArray pointers = new JSONArray();
            for (int i = 0; i < event.getPointerCount(); i++) {
                final JSONObject axesObj = new JSONObject();
                for (final InputDevice.MotionRange motionRange : motionRanges) {
                    final int axis = motionRange.getAxis();
                    axesObj.put(MotionEvent.axisToString(axis), event.getAxisValue(axis));
                }
                pointers.put(axesObj);
            }
            eventObj.put(KEY_POINTER_AXES, pointers);
            return eventObj.toString();
        } catch (JSONException e) {
            e.printStackTrace();
        }
        return "";
    }

    private static List<String> getStringsForSource(int source) {
        final List<String> sources = new ArrayList<>();
        sSourcePairs.forEach(sourcePair -> {
            final int sourceMask = sourcePair.first;
            final String sourceString = sourcePair.second;
            if ((source & sourceMask) != 0) {
                sources.add(sourceString);
            }
        });
        return sources;
    }
}
