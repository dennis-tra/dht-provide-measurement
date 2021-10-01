import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import plotly.graph_objects as go

events = pd.read_csv('plot_data.csv')

# plt.xlim(0, 90000)
# for index, row in events.iterrows():
#     distance = int(row['distance'], base=16)
#     distance_norm = distance / 2**256
#     plt.axhline(y=distance_norm, xmin=row['start']/90000, xmax=row['end']/90000, color=row['color'], linestyle='-')
# plt.yscale('log')
# plt.show()
# import plotly.express as px
# data_canada = px.data.gapminder().query("country == 'Canada'")
# fig = px.bar(data_canada, x='year', y='pop')
# fig.add_hline(y=0.9)
# fig.show()

import plotly.express as px

norm_distance = []
for index, row in events.iterrows():
    distance = int(row['distance'], base=16)
    norm_distance += [distance / 2 ** 256]
events['norm_distance'] = norm_distance

# events = events[events["type"] != "*main.OpenStreamStart"]
# events = events[events["type"] != "*main.OpenStreamEnd"]
events = events[events["type"] != "*main.OpenedStream"]
events = events[events["type"] != "*main.ClosedStream"]
events = events[events["type"] != "*main.ConnectedEvent"]
events = events[events["type"] != "*main.DisconnectedEvent"]

fig = go.Figure()

min_times = {}
for index, row in events.iterrows():
    peer_id = row["peer_id"]
    time = row["time"]
    if peer_id in min_times:
        if time < min_times[peer_id]:
            min_times[peer_id] = time
    else:
        min_times[peer_id] = time

peers = []
times = []
for key in min_times:
    peers += [key]
    times += [min_times[key]]

ordered_peers = [x for _, x in sorted(zip(times, peers))]
peer_order = {}
i = 0
for peer in ordered_peers:
    peer_order[peer] = len(ordered_peers) - i
    i += 1

dial_states = {}

# Plot Dials!
for index, row in events.iterrows():
    peer_id = row["peer_id"]
    event_type = row["type"]
    time = row["time"]
    has_error = row["has_error"]
    error = row["error"]
    extra = row["extra"]

    if event_type == "*main.DialStart":
        if peer_id not in dial_states:
            dial_states[peer_id] = {
                "time": time,
                "counter": 1,
            }
        else:
            dial_states[peer_id]["counter"] += 1
    elif event_type == "*main.ConnectedEvent":
        if peer_id not in dial_states:
            continue

    elif event_type == "*main.DialEnd":
        if peer_id not in dial_states:
            continue

        dial_states[peer_id]["counter"] -= 1

        if has_error and dial_states[peer_id]["counter"] != 0:
            continue

        if time - dial_states[peer_id]["time"] < 0.001:  # don't render dial shorter than 1ms
            dial_states.pop(row["peer_id"])
            continue

        color = "pink" if has_error else "red"

        trace = {
            'time': [dial_states[peer_id]["time"], time],
            'norm_distance': [row["norm_distance"], row["norm_distance"]],
            'order': [peer_order[peer_id] + 0.1, peer_order[peer_id] + 0.1],
        }
        duration_s = time - dial_states[peer_id]["time"]
        duration_str = "{:.3f}s".format(duration_s)
        if duration_s < 1:
            duration_str = "{:.1f}ms".format(duration_s * 1000)

        fig.add_trace(go.Scatter(
            x=trace["time"],
            y=trace["order"],
            name="Dialing Peer",
            marker=dict(
                size=3,
                color=[color, color]
            ),
            line=dict(
                color=color,
                width=2
            ),
            hovertemplate="Duration: {:s}<br>Multi Address: {:s}<br>Peer ID: {:s}<br>Error: {:s}".format(
                duration_str,
                extra, peer_id, str(error)[:30] if has_error else "-"),
            showlegend=False
        ))
        dial_states.pop(peer_id)

# Plot Requests!

request_states = {}

# Plot Dials!
for index, row in events.iterrows():
    peer_id = row["peer_id"]
    event_type = row["type"]
    time = row["time"]
    has_error = row["has_error"]
    error = row["error"]
    extra = row["extra"]

    if event_type == "*main.SendRequestStart":
        if peer_id not in request_states:
            request_states[peer_id] = {
                "time": time,
                "counter": 1,
            }
        else:
            dial_states[peer_id]["counter"] += 1

    elif event_type == "*main.SendRequestEnd":
        if peer_id not in request_states:
            continue

        request_states[peer_id]["counter"] -= 1

        if has_error and request_states[peer_id]["counter"] != 0:
            continue

        if time - request_states[peer_id]["time"] < 0.001:  # don't render dial shorter than 1ms
            request_states.pop(row["peer_id"])
            continue

        color = "lightgreen" if has_error else "green"

        trace = {
            'time': [request_states[peer_id]["time"], time],
            'norm_distance': [row["norm_distance"], row["norm_distance"]],
            'order': [peer_order[peer_id], peer_order[peer_id]],
        }
        duration_s = time - request_states[peer_id]["time"]
        duration_str = "{:.3f}s".format(duration_s)
        if duration_s < 1:
            duration_str = "{:.1f}ms".format(duration_s * 1000)

        fig.add_trace(go.Scatter(
            x=trace["time"],
            y=trace["order"],
            name="Requesting Peers",
            marker=dict(
                size=3,
                color=[color, color]
            ),
            line=dict(
                color=color,
                width=2
            ),
            hovertemplate="Duration: {:s}<br>Peer ID: {:s}<br>Error: {:s}".format(
                duration_str, peer_id, str(error)[:30] if has_error else "-"),
            showlegend=False
        ))
        request_states.pop(peer_id)

stream_states = {}

# Plot Open Stream!
for index, row in events.iterrows():
    peer_id = row["peer_id"]
    event_type = row["type"]
    time = row["time"]
    has_error = row["has_error"]
    error = row["error"]
    extra = row["extra"]

    if event_type == "*main.OpenStreamStart":
        if peer_id not in stream_states:
            stream_states[peer_id] = {
                "time": time,
                "counter": 1,
            }
        else:
            dial_states[peer_id]["counter"] += 1

    elif event_type == "*main.OpenStreamEnd":
        if peer_id not in stream_states:
            continue

        stream_states[peer_id]["counter"] -= 1

        if has_error and stream_states[peer_id]["counter"] != 0:
            continue

        if time - stream_states[peer_id]["time"] < 0.001:  # don't render dial shorter than 1ms
            stream_states.pop(row["peer_id"])
            continue

        color = "lightblue" if has_error else "blue"

        trace = {
            'time': [stream_states[peer_id]["time"], time],
            'norm_distance': [row["norm_distance"], row["norm_distance"]],
            'order': [peer_order[peer_id]+0.3, peer_order[peer_id]+0.3],
        }
        duration_s = time - stream_states[peer_id]["time"]
        duration_str = "{:.3f}s".format(duration_s)
        if duration_s < 1:
            duration_str = "{:.1f}ms".format(duration_s * 1000)

        fig.add_trace(go.Scatter(
            x=trace["time"],
            y=trace["order"],
            name="Opening Stream",
            marker=dict(
                size=3,
                color=[color, color]
            ),
            line=dict(
                color=color,
                width=2
            ),
            hovertemplate="Duration: {:s}<br>Peer ID: {:s}<br>Error: {:s}<br>Protocols: {:s}".format(
                duration_str, peer_id, str(error)[:30] if has_error else "-", extra),
            showlegend=False
        ))
        stream_states.pop(peer_id)

message_states = {}

# Plot Open Stream!
for index, row in events.iterrows():
    peer_id = row["peer_id"]
    event_type = row["type"]
    time = row["time"]
    has_error = row["has_error"]
    error = row["error"]
    extra = row["extra"]

    if event_type == "*main.SendMessageStart":
        if peer_id not in message_states:
            message_states[peer_id] = {
                "time": time,
                "counter": 1,
            }
        else:
            dial_states[peer_id]["counter"] += 1

    elif event_type == "*main.SendMessageEnd":
        if peer_id not in message_states:
            continue

        message_states[peer_id]["counter"] -= 1

        if has_error and message_states[peer_id]["counter"] != 0:
            continue

        if time - message_states[peer_id]["time"] < 0.001:  # don't render dial shorter than 1ms
            message_states.pop(row["peer_id"])
            continue

        color = "plum" if has_error else "purple"

        trace = {
            'time': [message_states[peer_id]["time"], time],
            'norm_distance': [row["norm_distance"], row["norm_distance"]],
            'order': [peer_order[peer_id]+0.2, peer_order[peer_id]+0.2],
        }
        duration_s = time - message_states[peer_id]["time"]
        duration_str = "{:.3f}s".format(duration_s)
        if duration_s < 1:
            duration_str = "{:.1f}ms".format(duration_s * 1000)

        fig.add_trace(go.Scatter(
            x=trace["time"],
            y=trace["order"],
            name="Adding Provider",
            marker=dict(
                size=3,
                color=[color, color]
            ),
            line=dict(
                color=color,
                width=2
            ),
            hovertemplate="Duration: {:s}<br>Peer ID: {:s}<br>Error: {:s}".format(
                duration_str, peer_id, str(error)[:30] if has_error else "-"),
            showlegend=False
        ))
        message_states.pop(peer_id)

# fig.update_yaxes(type="log")
fig.show()
