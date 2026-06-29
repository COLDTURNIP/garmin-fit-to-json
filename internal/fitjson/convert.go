package fitjson

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/kit/datetime"
	"github.com/muktihari/fit/kit/scaleoffset"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"
)

type Output struct {
	Metadata Metadata       `json:"metadata"`
	Summary  map[string]any `json:"summary"`
	Timeline Timeline       `json:"timeline"`
	Series   Series         `json:"series"`
}

type Metadata struct {
	InputFile        string         `json:"input_file"`
	MessageCountsRaw map[string]int `json:"message_counts_raw"`
	RecordCount      int            `json:"record_count"`
	LapCount         int            `json:"lap_count"`
	EventCount       int            `json:"event_count"`
}

type Timeline struct {
	Laps   []map[string]any `json:"laps"`
	Events []map[string]any `json:"events"`
}

type Series struct {
	Record []map[string]any `json:"record"`
}

var keepMessages = map[string]bool{
	"file_id":  true,
	"sport":    true,
	"session":  true,
	"lap":      true,
	"event":    true,
	"record":   true,
	"activity": true,
}

var keepRecordFields = fieldSet(
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
)

var keepLapFields = fieldSet(
	"start_time", "total_timer_time", "total_elapsed_time", "total_distance",
	"avg_heart_rate", "max_heart_rate", "avg_power", "max_power",
	"avg_cadence", "max_cadence", "normalized_power",
	"enhanced_avg_speed", "enhanced_max_speed",
	"event", "event_type", "lap_trigger", "message_index", "sport",
)

var keepSessionFields = fieldSet(
	"start_time", "total_elapsed_time", "total_timer_time", "total_distance",
	"sport", "sub_sport",
	"avg_heart_rate", "max_heart_rate",
	"avg_power", "max_power", "normalized_power",
	"avg_cadence", "max_cadence",
	"training_stress_score", "intensity_factor",
)

var keepEventFields = fieldSet("timestamp", "event", "event_type", "data")

var routedEvents = map[any]bool{
	"timer":             true,
	"lap":               true,
	"workout":           true,
	"front_gear_change": true,
	"rear_gear_change":  true,
}

func Convert(inputName string, r io.Reader) (*Output, error) {
	out := &Output{
		Metadata: Metadata{
			InputFile:        inputName,
			MessageCountsRaw: map[string]int{},
		},
		Summary:  map[string]any{},
		Timeline: Timeline{Laps: []map[string]any{}, Events: []map[string]any{}},
		Series:   Series{Record: []map[string]any{}},
	}

	dec := decoder.New(r)
	for dec.Next() {
		fit, err := dec.Decode()
		if err != nil {
			return nil, fmt.Errorf("decode FIT: %w", err)
		}
		for _, msg := range fit.Messages {
			name := messageName(msg.Num)
			out.Metadata.MessageCountsRaw[name]++

			if !keepMessages[name] {
				continue
			}

			switch name {
			case "session":
				values := extractValues(msg, keepSessionFields)
				if len(out.Summary) == 0 {
					out.Summary = values
				}
			case "lap":
				out.Timeline.Laps = append(out.Timeline.Laps, extractValues(msg, keepLapFields))
			case "event":
				values := extractValues(msg, keepEventFields)
				if routedEvents[values["event"]] {
					out.Timeline.Events = append(out.Timeline.Events, values)
				}
			case "record":
				out.Series.Record = append(out.Series.Record, extractValues(msg, keepRecordFields))
			}
		}
	}

	enrichLapHeartRates(out)

	out.Metadata.RecordCount = len(out.Series.Record)
	out.Metadata.LapCount = len(out.Timeline.Laps)
	out.Metadata.EventCount = len(out.Timeline.Events)
	return out, nil
}

func ConvertFile(inputPath, outputPath string) error {
	in, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer in.Close()

	out, err := Convert(filepath.Base(inputPath), in)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer file.Close()

	if err := Write(file, out); err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}

func Write(w io.Writer, out *Output) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

var pythonUnknownMessages = map[typedef.MesgNum]string{
	typedef.MesgNum(13):  "unknown_13",
	typedef.MesgNum(216): "unknown_216",
	typedef.MesgNum(313): "unknown_313",
}

func messageName(num typedef.MesgNum) string {
	if name, ok := pythonUnknownMessages[num]; ok {
		return name
	}
	name := num.String()
	const prefix = "MesgNumInvalid("
	if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ")") {
		decimal := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ")")
		if _, err := strconv.ParseUint(decimal, 10, 16); err == nil {
			return "unknown_" + decimal
		}
	}
	return name
}

func extractValues(msg proto.Message, keep map[string]bool) map[string]any {
	values := map[string]any{}
	for _, field := range msg.Fields {
		name := field.Name
		scale := field.Scale
		offset := field.Offset
		if sub := field.SubFieldSubstitution(&msg); sub != nil {
			name = sub.Name
			scale = sub.Scale
			offset = sub.Offset
		}
		if !keep[name] {
			continue
		}
		values[name] = normalizeValue(msg, field, name, scale, offset)
	}
	return values
}

const jsonTimeLayout = "2006-01-02T15:04:05"

func enrichLapHeartRates(out *Output) {
	for i := range out.Timeline.Laps {
		start, ok := timeFromMap(out.Timeline.Laps[i], "start_time")
		if !ok {
			out.Timeline.Laps[i]["start_heart_rate"] = 0
			out.Timeline.Laps[i]["end_heart_rate"] = 0
			continue
		}

		end, hasEnd, ok := lapEndTime(out.Timeline.Laps, i, start)
		if !ok {
			out.Timeline.Laps[i]["start_heart_rate"] = 0
			out.Timeline.Laps[i]["end_heart_rate"] = 0
			continue
		}
		startHeartRate := 0
		endHeartRate := 0
		for _, record := range out.Series.Record {
			recordTime, ok := timeFromMap(record, "timestamp")
			if !ok || recordTime.Before(start) {
				continue
			}
			if hasEnd && !recordTime.Before(end) {
				continue
			}
			heartRate, ok := intValue(record["heart_rate"])
			if !ok || heartRate <= 0 {
				continue
			}
			if startHeartRate == 0 {
				startHeartRate = heartRate
			}
			endHeartRate = heartRate
		}
		out.Timeline.Laps[i]["start_heart_rate"] = startHeartRate
		out.Timeline.Laps[i]["end_heart_rate"] = endHeartRate
	}
}

func lapEndTime(laps []map[string]any, i int, start time.Time) (time.Time, bool, bool) {
	if i+1 < len(laps) {
		end, ok := timeFromMap(laps[i+1], "start_time")
		return end, ok, ok
	}
	if seconds, ok := floatSeconds(laps[i]["total_elapsed_time"]); ok {
		return start.Add(time.Duration(seconds * float64(time.Second))), true, true
	}
	if seconds, ok := floatSeconds(laps[i]["total_timer_time"]); ok {
		return start.Add(time.Duration(seconds * float64(time.Second))), true, true
	}
	return time.Time{}, false, true
}

func timeFromMap(values map[string]any, key string) (time.Time, bool) {
	value, ok := values[key].(string)
	if !ok {
		return time.Time{}, false
	}
	t, err := time.Parse(jsonTimeLayout, value)
	return t, err == nil
}

func floatSeconds(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}

func intValue(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		if int64(int(v)) != v {
			return 0, false
		}
		return int(v), true
	case uint:
		if uint(int(v)) != v {
			return 0, false
		}
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		if uint32(int(v)) != v {
			return 0, false
		}
		return int(v), true
	case uint64:
		if uint64(int(v)) != v {
			return 0, false
		}
		return int(v), true
	default:
		return 0, false
	}
}

func normalizeValue(msg proto.Message, field proto.Field, name string, scale, offset float64) any {
	_ = msg
	if !field.Value.Valid(field.BaseType) {
		return nil
	}
	switch name {
	case "timestamp", "start_time", "time_created":
		return datetime.ToTime(field.Value.Uint32()).UTC().Format(jsonTimeLayout)
	case "event":
		return typedef.Event(field.Value.Uint8()).String()
	case "event_type":
		return typedef.EventType(field.Value.Uint8()).String()
	case "lap_trigger":
		return typedef.LapTrigger(field.Value.Uint8()).String()
	case "sport":
		return typedef.Sport(field.Value.Uint8()).String()
	case "sub_sport":
		return typedef.SubSport(field.Value.Uint8()).String()
	default:
		return scaleoffset.ApplyValue(field.Value, scale, offset).Any()
	}
}

func fieldSet(names ...string) map[string]bool {
	fields := make(map[string]bool, len(names))
	for _, name := range names {
		fields[name] = true
	}
	return fields
}
