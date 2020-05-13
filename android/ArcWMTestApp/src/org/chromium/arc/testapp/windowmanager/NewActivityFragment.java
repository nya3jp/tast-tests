/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.windowmanager;

import android.app.ActivityOptions;
import android.app.Dialog;
import android.app.Fragment;
import android.content.Context;
import android.content.Intent;
import android.graphics.Rect;
import android.hardware.display.DisplayManager;
import android.os.Build;
import android.os.Bundle;
import android.util.Log;
import android.view.Display;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
import android.widget.AdapterView;
import android.widget.ArrayAdapter;
import android.widget.Button;
import android.widget.CheckBox;
import android.widget.ListView;
import android.widget.RadioGroup;
import android.widget.TextView;

import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;

/**
 * A simple {@link Fragment} subclass.
 * It handles the logic (the controller) for "Options for New Activity" like:
 * <ul>
 *     <li>setting the orientation</li>
 *     <li>enabling immersive mode</li>
 *     <li>creating a root activity</li>
 *     <li>setting the launch display</li>
 *     <li>setting the launch bounds</li>
 * </ul>
 *
 * @see CurrentActivityFragment
 */
public class NewActivityFragment extends Fragment {
    private static final String TAG = "ArcWMTestApp";

    private static final Rect LAUNCH_BOUNDS = new Rect(50, 50, 1050, 1050);

    private Method mSetLaunchDisplayId = null;

    private int mLaunchDisplay = Display.INVALID_DISPLAY;
    private Rect mLaunchBounds = null;

    public NewActivityFragment() {
        try {
            // Use reflection to access a hidden API backported from O.
            final Class<ActivityOptions> clazz = ActivityOptions.class;
            mSetLaunchDisplayId = clazz.getMethod("setLaunchDisplayId", int.class);
        } catch (NoSuchMethodException e) {
            Log.i(TAG, "setLaunchDisplayId API is unavailable.");
        }
    }

    @Override
    public View onCreateView(
            LayoutInflater inflater, ViewGroup container, Bundle savedInstanceState) {
        // Inflate the layout for this fragment
        final View rootView = inflater.inflate(R.layout.fragment_new_activity, container, false);

        final Context context = getActivity().getApplicationContext();
        final DisplayManager displayManager =
                (DisplayManager) context.getSystemService(Context.DISPLAY_SERVICE);
        final String unspecified = context.getResources().getString(R.string.unspecified);
        final boolean enableDisplayView =
                Build.VERSION.SDK_INT > Build.VERSION_CODES.M && mSetLaunchDisplayId != null;

        final TextView displayView = (TextView) rootView.findViewById(R.id.text_view_set_display);
        displayView.setEnabled(enableDisplayView);
        if (enableDisplayView) {
            displayView.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View view) {
                    final Context context = view.getContext();
                    final Dialog dialog = new Dialog(context);
                    dialog.setContentView(R.layout.list_dialog);

                    final Display[] displays = displayManager.getDisplays();
                    final String[] items = new String[displays.length + 1];

                    items[0] = unspecified;
                    for (int i = 0; i < displays.length; i++) {
                        Display display = displays[i];
                        items[i + 1] = display.getDisplayId() + " (" + display.getName() + ")";
                    }

                    final ListView listView = (ListView) dialog.findViewById(R.id.list_view);
                    listView.setAdapter(new ArrayAdapter<>(
                            context, android.R.layout.simple_list_item_1, items));

                    listView.setOnItemClickListener(new AdapterView.OnItemClickListener() {
                        public void onItemClick(
                                AdapterView parent, View view, int position, long id) {
                            final TextView textView = (TextView) view;
                            displayView.setText(textView.getText());
                            // The first item is "Unspecified", followed by displays in order.
                            mLaunchDisplay = position == 0 ? Display.INVALID_DISPLAY
                                                           : displays[position - 1].getDisplayId();
                            dialog.dismiss();
                        }
                    });

                    dialog.show();
                }
            });
        }

        final boolean enableBoundsView = Build.VERSION.SDK_INT > Build.VERSION_CODES.M;

        final TextView boundsView = (TextView) rootView.findViewById(R.id.text_view_set_bounds);
        boundsView.setEnabled(enableBoundsView);
        if (enableBoundsView) {
            boundsView.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View view) {
                    final Context context = view.getContext();
                    final Dialog dialog = new Dialog(context);
                    dialog.setContentView(R.layout.list_dialog);

                    final ListView listView = (ListView) dialog.findViewById(R.id.list_view);
                    listView.setAdapter(new ArrayAdapter<>(context,
                            android.R.layout.simple_list_item_1,
                            new String[] {context.getResources().getString(R.string.unspecified),
                                    LAUNCH_BOUNDS.toString()}));

                    listView.setOnItemClickListener(new AdapterView.OnItemClickListener() {
                        public void onItemClick(
                                AdapterView parent, View view, int position, long id) {
                            final TextView textView = (TextView) view;
                            boundsView.setText(textView.getText());
                            mLaunchBounds = position == 0 ? null : LAUNCH_BOUNDS;
                            dialog.dismiss();
                        }
                    });

                    dialog.show();
                }
            });
        }

        Button launchButton = (Button) rootView.findViewById(R.id.button_launch_activity);
        launchButton.setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                final boolean rootActivity =
                        ((CheckBox) (rootView.findViewById(R.id.check_box_root_activity)))
                                .isChecked();
                final boolean hideSystemBar =
                        ((CheckBox) (rootView.findViewById(R.id.check_box_hide_system_bar)))
                                .isChecked();
                final int checkedId =
                        ((RadioGroup) (rootView.findViewById(R.id.radio_group_orientation)))
                                .getCheckedRadioButtonId();

                Intent intent = null;

                if (checkedId == R.id.radio_button_landscape) {
                    if (hideSystemBar) {
                        intent = new Intent(context, MainLandscapeImmersiveActivity.class);
                    } else {
                        intent = new Intent(context, MainLandscapeActivity.class);
                    }
                } else if (checkedId == R.id.radio_button_portrait) {
                    if (hideSystemBar) {
                        intent = new Intent(context, MainPortraitImmersiveActivity.class);
                    } else {
                        intent = new Intent(context, MainPortraitActivity.class);
                    }
                } else if (checkedId == R.id.radio_button_unspecified) {
                    if (hideSystemBar) {
                        intent = new Intent(context, MainUnspecifiedImmersiveActivity.class);
                    } else {
                        intent = new Intent(context, MainActivity.class);
                    }
                }

                if (intent != null) {
                    if (rootActivity) {
                        intent.setFlags(
                                Intent.FLAG_ACTIVITY_CLEAR_TASK | Intent.FLAG_ACTIVITY_NEW_TASK);
                        intent.putExtra(BaseActivity.EXTRA_ACTIVITY_NUMBER, 1);
                    } else {
                        intent.setFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                        final int activityNumber =
                                ((BaseActivity) getActivity()).getActivityNumber();
                        intent.putExtra(BaseActivity.EXTRA_ACTIVITY_NUMBER, activityNumber + 1);
                    }

                    final ActivityOptions options = ActivityOptions.makeBasic();

                    if (mSetLaunchDisplayId != null && mLaunchDisplay != Display.INVALID_DISPLAY) {
                        try {
                            mSetLaunchDisplayId.invoke(options, mLaunchDisplay);
                        } catch (IllegalAccessException | IllegalArgumentException
                                | InvocationTargetException e) {
                            Log.e(TAG, "Failure setting launch display.", e);
                        }
                    }

                    if (mLaunchBounds != null) {
                        try {
                            // Use reflection to compile N API against M SDK.
                            final Class<ActivityOptions> clazz = ActivityOptions.class;
                            final Method method = clazz.getMethod("setLaunchBounds", Rect.class);
                            method.invoke(options, mLaunchBounds);
                        } catch (NoSuchMethodException | IllegalAccessException
                                | IllegalArgumentException | InvocationTargetException e) {
                            Log.e(TAG, "Failure setting launch bounds.", e);
                        }
                    }

                    startActivity(intent, options.toBundle());
                }
            }
        });

        return rootView;
    }
}
