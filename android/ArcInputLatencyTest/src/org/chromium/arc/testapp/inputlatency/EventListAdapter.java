/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.inputlatency;

import android.content.Context;
import android.view.LayoutInflater;
import android.view.View;
import android.view.ViewGroup;
import android.widget.BaseAdapter;
import android.widget.TextView;

import java.util.List;

class EventListAdapter extends BaseAdapter {
    private Context mContext;
    private List<ReceivedEvent> mList = null;

    public EventListAdapter(Context context, List<ReceivedEvent> list) {
        mContext = context;
        mList = list;
    }

    @Override
    public View getView(int position, View convertView, ViewGroup parent) {
        View view = convertView;
        if (view == null)
            view = LayoutInflater.from(mContext).inflate(R.layout.event_list_item, null);
        final ReceivedEvent item = getItem(position);
        ((TextView) view.findViewById(R.id.event_type)).setText(item.type);
        ((TextView) view.findViewById(R.id.event_time)).setText(item.eventTime.toString());
        ((TextView) view.findViewById(R.id.app_time)).setText(item.receiveTime.toString());
        ((TextView) view.findViewById(R.id.latency)).setText(item.latency.toString());
        return view;
    }

    @Override
    public ReceivedEvent getItem(int position) {
        return mList.get(position);
    }

    @Override
    public long getItemId(int position) {
        return mList.get(position).event.getEventTime();
    }

    @Override
    public int getCount() {
        return mList.size();
    }
}
