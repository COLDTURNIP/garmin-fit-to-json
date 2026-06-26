import sys
import json
from fitparse import FitFile

KEEP_MESSAGE_TYPES = {
    "file_id", "sport", "session", "lap", "event", "record", "activity"
}

KEEP_RECORD_FIELDS = {
    "timestamp",
    "power",
    "heart_rate",
    "cadence",
    "distance",
    "enhanced_speed",
    "speed",
    "altitude",
    "enhanced_altitude",
    "temperature",
    "accumulated_power",
    "left_right_balance",
    "resistance",
}

KEEP_LAP_FIELDS = {
    "start_time", "total_timer_time", "total_elapsed_time", "total_distance",
    "avg_heart_rate", "max_heart_rate", "avg_power", "max_power",
    "avg_cadence", "max_cadence", "normalized_power",
    "enhanced_avg_speed", "enhanced_max_speed",
    "event", "event_type", "lap_trigger", "message_index", "sport"
}

KEEP_SESSION_FIELDS = {
    "start_time", "total_elapsed_time", "total_timer_time", "total_distance",
    "sport", "sub_sport",
    "avg_heart_rate", "max_heart_rate",
    "avg_power", "max_power", "normalized_power",
    "avg_cadence", "max_cadence",
    "training_stress_score", "intensity_factor"
}

KEEP_EVENT_FIELDS = {"timestamp", "event", "event_type", "data"}

def norm(v):
    return v.isoformat() if hasattr(v, "isoformat") else v

def extract_values(msg, keep_fields=None):
    out = {}
    for field in msg:
        if keep_fields is None or field.name in keep_fields:
            out[field.name] = norm(field.value)
    return out

def fit_to_analysis_json(input_path, output_path):
    fitfile = FitFile(input_path)

    result = {
        "metadata": {"input_file": input_path},
        "summary": {},
        "timeline": {"laps": [], "events": []},
        "series": {"record": []},
    }

    message_counts = {}

    for msg in fitfile.get_messages():
        name = msg.name
        message_counts[name] = message_counts.get(name, 0) + 1

        if name not in KEEP_MESSAGE_TYPES:
            continue

        if name == "session":
            values = extract_values(msg, KEEP_SESSION_FIELDS)
            if not result["summary"]:
                result["summary"] = values

        elif name == "lap":
            result["timeline"]["laps"].append(extract_values(msg, KEEP_LAP_FIELDS))

        elif name == "event":
            ev = extract_values(msg, KEEP_EVENT_FIELDS)
            if ev.get("event") in {"timer", "lap", "workout", "front_gear_change", "rear_gear_change"}:
                result["timeline"]["events"].append(ev)

        elif name == "record":
            result["series"]["record"].append(extract_values(msg, KEEP_RECORD_FIELDS))

    result["metadata"]["message_counts_raw"] = message_counts
    result["metadata"]["record_count"] = len(result["series"]["record"])
    result["metadata"]["lap_count"] = len(result["timeline"]["laps"])
    result["metadata"]["event_count"] = len(result["timeline"]["events"])

    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(result, f, ensure_ascii=False, indent=2)

if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} input.fit output.json", file=sys.stderr)
        sys.exit(1)

    fit_to_analysis_json(sys.argv[1], sys.argv[2])
