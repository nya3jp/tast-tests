/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.windowmanager;

import android.util.Log;

import org.json.JSONException;
import org.json.JSONStringer;

/**
 * Helper class that has all the valid keys used in the JSON string
 * and other helper functions.
 */
class JsonHelper {
    private static final String TAG = "JsonHelper";

    // JSON keys used in the Captions TextView
    static final String JSON_KEY_ACTIVITY_NR = "activityNr";
    static final String JSON_KEY_BUTTON = "buttons";
    static final String JSON_KEY_CAPTION_VISIBILITY = "captionVisibility";
    static final String JSON_KEY_DEVICE_MODE = "deviceMode";
    static final String JSON_KEY_ACCEL = "accel";
    static final String JSON_KEY_ORIENTATION = "orientation";
    static final String JSON_KEY_ROTATION = "rotation";
    static final String JSON_KEY_WINDOW_STATE = "windowState";
    static final String JSON_KEY_ZOOMED = "zoomed";

    private static final String JSON_KEY_ERROR = "error";

    /**
     * Generates an error string for a given {@code stringId}
     *
     * @param errorDescription The error message
     * @return the error string in JSON format
     */
    static String reportError(String errorDescription) {
        JSONStringer js = new JSONStringer();
        try {
            js.object().key(JSON_KEY_ERROR).value(errorDescription);
            js.endObject();
        } catch (JSONException e) {
            Log.w(TAG, "Error creating JSONStringer", e);
        }
        return js.toString();
    }

    /**
     * Converts a caption visibility to string.
     *
     * @param captionVisibility the integer to be converted to {@code String}
     * @return the string representation of the captionVisibility.
     * @see org.chromium.arc.CaptionConfiguration.CAPTION_XXX
     */
    static String toCaptionVisibilityString(int captionVisibility) {
        switch (captionVisibility) {
            case 0: // DOES_NOT_EXIST
                return "none";
            case 1: // AUTO_HIDE
            case 3: // AUTO_HIDE_BY_SYSTEM
                return "auto_hide";
            case 2: // VISIBLE
            case 4: // VISIBLE_BY_SYSTEM;
                return "visible";
        }
        return "error";
    }
}
