import pandas as pd
import plotly.graph_objects as go

# read events CSV
events = pd.read_csv('events.csv')

# Norm the XOR distance to a value between 0 and 1
norm_distance = []
for index, row in events.iterrows():
    distance = int(row['distance'], base=16)
    norm_distance += [distance / 2 ** 256]
events['norm_distance'] = norm_distance

min_times = {}
for index, row in events.iterrows():
    peer_id = row["peer_id"]
    time = row["time"]
    event_type = row["type"]
    if "Monitor" in event_type:
        continue

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

fig = go.Figure()


def hex_to_rgba(h, alpha):
    """
    converts color value in hex format to rgba format with alpha transparency
    """
    return tuple([int(h.lstrip('#')[i:i + 2], 16) for i in (0, 2, 4)] + [alpha])


def track_start_event(states, peer_id):
    if peer_id not in states:
        states[peer_id] = {
            "time": time,
            "counter": 1,
        }
    else:
        states[peer_id]["counter"] += 1


def handle_end_event(states, event, color, name, offset):
    peer_id = event["peer_id"]
    event_type = event["type"]
    time = event["time"]
    has_error = event["has_error"]
    error = event["error"]
    extra = event["extra"]
    norm_distance = event["norm_distance"]

    if peer_id not in states:
        return

    states[peer_id]["counter"] -= 1

    if event["has_error"] and states[peer_id]["counter"] != 0:
        return

    if event_type == "*main.MonitorProviderEnd" and has_error:
        states.pop(peer_id)
        return

    color = 'rgba' + str(hex_to_rgba(color, 0.2) if has_error else hex_to_rgba(color, 1))

    trace = {
        'time': [states[peer_id]["time"], time],
        'norm_distance': [row["norm_distance"], row["norm_distance"]],
        'order': [peer_order[peer_id] - offset, peer_order[peer_id] - offset],
    }

    duration_in_s = time - states[peer_id]["time"]
    duration_str = "{:.3f}s".format(duration_in_s) if duration_in_s >= 1 else "{:.1f}ms".format(duration_in_s * 1000)

    fig.add_trace(go.Scatter(
        x=trace["time"],
        y=trace["order"],
        name=name,
        marker=dict(
            size=8,
            color=[color, color],
            line_color=[color, color],
            symbol=["triangle-right", "diamond" if has_error else "triangle-left"]
        ),
        line=dict(
            color=color,
            width=5,
        ),
        hovertemplate="Start: {:.5f}<br>"
                      "End: {:.5f}<br>"
                      "Duration: {:s}<br>"
                      "||XOR||: {:.4e}<br>"
                      "Peer ID: {:s}<br>"
                      "Error: {:s}<br"
                      ">Extra: {:s}".format(
            states[peer_id]["time"],
            time,
            duration_str,
            norm_distance,
            peer_id,
            str(error)[:30] if has_error else "",
            str(extra)),
        showlegend=False
    ))
    states.pop(peer_id)


dial_states = {}
message_states = {}
request_states = {}
stream_states = {}
monitor_states = {}

for index, row in events.iterrows():
    peer_id = row["peer_id"]
    event_type = row["type"]
    time = row["time"]
    has_error = row["has_error"]
    error = row["error"]
    extra = row["extra"]

    if event_type == "*main.SendRequestStart":
        track_start_event(request_states, peer_id)

    elif event_type == "*main.SendRequestEnd":
        handle_end_event(request_states, row, "#177eef", "Finding Closer Nodes", 0.5)

    elif event_type == "*main.DialStart":
        track_start_event(dial_states, peer_id)

    elif event_type == "*main.DialEnd" or event_type == "*main.ConnectedEvent":
        handle_end_event(dial_states, row, "#d62728", "Dialing Peer", 0.5)

    elif event_type == "*main.OpenStreamStart":
        pass
        # track_start_event(stream_states, peer_id)

    elif event_type == "*main.OpenStreamEnd" or event_type == "*main.OpenedStream":
        pass
        # handle_end_event(stream_states, row, "blue", "lightblue", "Opening Stream", 0.3)

    elif event_type == "*main.SendMessageStart":
        track_start_event(message_states, peer_id)

    elif event_type == "*main.SendMessageEnd":
        handle_end_event(message_states, row, "#9467bd", "Adding Provider", 0.5)

    elif event_type == "*main.MonitorProviderStart":
        track_start_event(monitor_states, peer_id)

    elif event_type == "*main.MonitorProviderEnd":
        handle_end_event(monitor_states, row, "#000000", "Monitoring Provider", 0.5)

for i in range(len(ordered_peers)):

    fig.add_annotation(x=min_times[ordered_peers[0]] - 10, y=i + 0.5,
                       text=ordered_peers[len(ordered_peers) - i - 1][:16], showarrow=False)
    if i % 2 == 0:
        fig.add_hrect(y0=i, y1=i + 1, line_width=0, fillcolor="black", opacity=0.05)

fig.show()
