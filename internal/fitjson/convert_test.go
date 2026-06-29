package fitjson

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/muktihari/fit/encoder"
	"github.com/muktihari/fit/kit/datetime"
	"github.com/muktihari/fit/profile/factory"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/profile/untyped/fieldnum"
	"github.com/muktihari/fit/profile/untyped/mesgnum"
	"github.com/muktihari/fit/proto"
)

func sampleFIT(t *testing.T) []byte {
	t.Helper()

	start := time.Date(2021, 12, 30, 21, 52, 8, 0, time.UTC)
	fit := &proto.FIT{Messages: []proto.Message{
		{Num: mesgnum.FileId, Fields: []proto.Field{
			factory.CreateField(mesgnum.FileId, fieldnum.FileIdType).WithValue(typedef.FileActivity),
		}},
		{Num: mesgnum.Session, Fields: []proto.Field{
			factory.CreateField(mesgnum.Session, fieldnum.SessionStartTime).WithValue(datetime.ToUint32(start)),
			factory.CreateField(mesgnum.Session, fieldnum.SessionSport).WithValue(typedef.SportCycling),
			factory.CreateField(mesgnum.Session, fieldnum.SessionSubSport).WithValue(typedef.SubSportGeneric),
			factory.CreateField(mesgnum.Session, fieldnum.SessionAvgHeartRate).WithValue(uint8(150)),
			factory.CreateField(mesgnum.Session, fieldnum.SessionAvgPower).WithValue(uint16(210)),
		}},
		{Num: mesgnum.Lap, Fields: []proto.Field{
			factory.CreateField(mesgnum.Lap, fieldnum.LapStartTime).WithValue(datetime.ToUint32(start)),
			factory.CreateField(mesgnum.Lap, fieldnum.LapLapTrigger).WithValue(typedef.LapTriggerManual),
			factory.CreateField(mesgnum.Lap, fieldnum.LapAvgHeartRate).WithValue(uint8(151)),
			factory.CreateField(mesgnum.Lap, fieldnum.LapAvgPower).WithValue(uint16(211)),
			factory.CreateField(mesgnum.Lap, fieldnum.LapTotalElapsedTime).WithValue(uint32(3000)),
			factory.CreateField(mesgnum.Lap, fieldnum.LapTotalTimerTime).WithValue(uint32(3000)),
		}},
		{Num: mesgnum.Event, Fields: []proto.Field{
			factory.CreateField(mesgnum.Event, fieldnum.EventTimestamp).WithValue(datetime.ToUint32(start.Add(time.Second))),
			factory.CreateField(mesgnum.Event, fieldnum.EventEvent).WithValue(typedef.EventTimer),
			factory.CreateField(mesgnum.Event, fieldnum.EventEventType).WithValue(typedef.EventTypeStart),
		}},
		{Num: mesgnum.Event, Fields: []proto.Field{
			factory.CreateField(mesgnum.Event, fieldnum.EventTimestamp).WithValue(datetime.ToUint32(start.Add(2 * time.Second))),
			factory.CreateField(mesgnum.Event, fieldnum.EventEvent).WithValue(typedef.EventBattery),
			factory.CreateField(mesgnum.Event, fieldnum.EventEventType).WithValue(typedef.EventTypeMarker),
		}},
		{Num: mesgnum.Record, Fields: []proto.Field{
			factory.CreateField(mesgnum.Record, fieldnum.RecordTimestamp).WithValue(datetime.ToUint32(start)),
			factory.CreateField(mesgnum.Record, fieldnum.RecordHeartRate).WithValue(uint8(149)),
			factory.CreateField(mesgnum.Record, fieldnum.RecordCadence).WithValue(uint8(86)),
			factory.CreateField(mesgnum.Record, fieldnum.RecordPower).WithValue(uint16(200)),
			factory.CreateField(mesgnum.Record, fieldnum.RecordTemperature).WithValue(int8(20)),
		}},
		{Num: mesgnum.Record, Fields: []proto.Field{
			factory.CreateField(mesgnum.Record, fieldnum.RecordTimestamp).WithValue(datetime.ToUint32(start.Add(2 * time.Second))),
			factory.CreateField(mesgnum.Record, fieldnum.RecordHeartRate).WithValue(uint8(152)),
			factory.CreateField(mesgnum.Record, fieldnum.RecordCadence).WithValue(uint8(88)),
			factory.CreateField(mesgnum.Record, fieldnum.RecordPower).WithValue(uint16(250)),
			factory.CreateField(mesgnum.Record, fieldnum.RecordTemperature).WithValue(int8(21)),
		}},
	}}

	var buf bytes.Buffer
	if err := encoder.New(&buf).Encode(fit); err != nil {
		t.Fatalf("encode sample FIT: %v", err)
	}
	return buf.Bytes()
}

func TestConvertProducesFilteredJSONShape(t *testing.T) {
	out, err := Convert("sample.fit", bytes.NewReader(sampleFIT(t)))
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	if out.Metadata.InputFile != "sample.fit" {
		t.Fatalf("input_file = %q", out.Metadata.InputFile)
	}
	for name, want := range map[string]int{"file_id": 1, "session": 1, "lap": 1, "event": 2, "record": 2} {
		if got := out.Metadata.MessageCountsRaw[name]; got != want {
			t.Fatalf("message_counts_raw[%q] = %d, want %d", name, got, want)
		}
	}
	if out.Metadata.RecordCount != 2 || out.Metadata.LapCount != 1 || out.Metadata.EventCount != 1 {
		t.Fatalf("counts = record %d lap %d event %d", out.Metadata.RecordCount, out.Metadata.LapCount, out.Metadata.EventCount)
	}

	assertEqual(t, out.Summary["start_time"], "2021-12-30T21:52:08")
	assertEqual(t, out.Summary["sport"], "cycling")
	assertEqual(t, out.Summary["sub_sport"], "generic")
	assertEqual(t, out.Summary["avg_heart_rate"], uint8(150))
	assertEqual(t, out.Summary["avg_power"], uint16(210))

	if len(out.Timeline.Laps) != 1 {
		t.Fatalf("laps len = %d", len(out.Timeline.Laps))
	}
	assertEqual(t, out.Timeline.Laps[0]["lap_trigger"], "manual")
	assertEqual(t, out.Timeline.Laps[0]["avg_power"], uint16(211))
	assertEqual(t, out.Timeline.Laps[0]["start_heart_rate"], 149)
	assertEqual(t, out.Timeline.Laps[0]["end_heart_rate"], 152)

	if len(out.Timeline.Events) != 1 {
		t.Fatalf("events len = %d", len(out.Timeline.Events))
	}
	assertEqual(t, out.Timeline.Events[0]["event"], "timer")
	assertEqual(t, out.Timeline.Events[0]["event_type"], "start")

	if len(out.Series.Record) != 2 {
		t.Fatalf("record len = %d", len(out.Series.Record))
	}
	assertEqual(t, out.Series.Record[1]["timestamp"], "2021-12-30T21:52:10")
	assertEqual(t, out.Series.Record[1]["heart_rate"], uint8(152))
	assertEqual(t, out.Series.Record[1]["cadence"], uint8(88))
	assertEqual(t, out.Series.Record[1]["power"], uint16(250))
	assertEqual(t, out.Series.Record[1]["temperature"], int8(21))
}

func TestWriteUsesIndentedJSON(t *testing.T) {
	var buf bytes.Buffer
	out := &Output{Metadata: Metadata{InputFile: "sample.fit", MessageCountsRaw: map[string]int{}}, Summary: map[string]any{}, Timeline: Timeline{Laps: []map[string]any{}, Events: []map[string]any{}}, Series: Series{Record: []map[string]any{}}}
	if err := Write(&buf, out); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "\n  \"metadata\":") {
		t.Fatalf("JSON is not indented with two-space keys:\n%s", buf.String())
	}
}

func TestEnrichLapHeartRatesAddsZeroWhenNoHeartRateRecordMatches(t *testing.T) {
	out := &Output{
		Timeline: Timeline{Laps: []map[string]any{{
			"start_time":         "2021-12-30T21:52:08",
			"total_elapsed_time": 1.0,
			"total_timer_time":   1.0,
			"avg_heart_rate":     uint8(151),
			"max_heart_rate":     uint8(151),
		}}},
		Series: Series{Record: []map[string]any{{
			"timestamp": "2021-12-30T21:52:08",
			"cadence":   uint8(88),
		}}},
	}

	enrichLapHeartRates(out)

	assertEqual(t, out.Timeline.Laps[0]["start_heart_rate"], 0)
	assertEqual(t, out.Timeline.Laps[0]["end_heart_rate"], 0)
}

func TestEnrichLapHeartRatesUsesHalfOpenLapBoundaries(t *testing.T) {
	out := &Output{
		Timeline: Timeline{Laps: []map[string]any{
			{
				"start_time":         "2021-12-30T21:52:08",
				"total_elapsed_time": 3.0,
			},
			{
				"start_time":         "2021-12-30T21:52:11",
				"total_elapsed_time": 2.0,
			},
		}},
		Series: Series{Record: []map[string]any{
			{"timestamp": "2021-12-30T21:52:08", "heart_rate": uint8(149)},
			{"timestamp": "2021-12-30T21:52:10", "heart_rate": uint8(152)},
			{"timestamp": "2021-12-30T21:52:11", "heart_rate": uint8(153)},
			{"timestamp": "2021-12-30T21:52:12", "heart_rate": uint8(154)},
			{"timestamp": "2021-12-30T21:52:13", "heart_rate": uint8(155)},
		}},
	}

	enrichLapHeartRates(out)

	assertEqual(t, out.Timeline.Laps[0]["start_heart_rate"], 149)
	assertEqual(t, out.Timeline.Laps[0]["end_heart_rate"], 152)
	assertEqual(t, out.Timeline.Laps[1]["start_heart_rate"], 153)
	assertEqual(t, out.Timeline.Laps[1]["end_heart_rate"], 154)
}

func TestEnrichLapHeartRatesUsesTimerTimeFallbackAndSkipsNonPositiveRates(t *testing.T) {
	out := &Output{
		Timeline: Timeline{Laps: []map[string]any{{
			"start_time":       "2021-12-30T21:52:08",
			"total_timer_time": 3.0,
		}}},
		Series: Series{Record: []map[string]any{
			{"timestamp": "2021-12-30T21:52:08", "heart_rate": 0},
			{"timestamp": "2021-12-30T21:52:09", "heart_rate": int8(-1)},
			{"timestamp": "2021-12-30T21:52:10", "heart_rate": uint8(151)},
			{"timestamp": "2021-12-30T21:52:11", "heart_rate": uint8(152)},
		}},
	}

	enrichLapHeartRates(out)

	assertEqual(t, out.Timeline.Laps[0]["start_heart_rate"], 151)
	assertEqual(t, out.Timeline.Laps[0]["end_heart_rate"], 151)
}

func TestEnrichLapHeartRatesAddsZeroWhenLapStartIsInvalid(t *testing.T) {
	out := &Output{
		Timeline: Timeline{Laps: []map[string]any{{
			"start_time":         "invalid",
			"total_elapsed_time": 3.0,
		}}},
		Series: Series{Record: []map[string]any{{
			"timestamp":  "2021-12-30T21:52:08",
			"heart_rate": uint8(151),
		}}},
	}

	enrichLapHeartRates(out)

	assertEqual(t, out.Timeline.Laps[0]["start_heart_rate"], 0)
	assertEqual(t, out.Timeline.Laps[0]["end_heart_rate"], 0)
}

func TestEnrichLapHeartRatesAddsZeroWhenNextLapStartIsInvalid(t *testing.T) {
	out := &Output{
		Timeline: Timeline{Laps: []map[string]any{
			{
				"start_time":         "2021-12-30T21:52:08",
				"total_elapsed_time": 3.0,
			},
			{
				"start_time":         "invalid",
				"total_elapsed_time": 3.0,
			},
		}},
		Series: Series{Record: []map[string]any{
			{"timestamp": "2021-12-30T21:52:08", "heart_rate": uint8(149)},
			{"timestamp": "2021-12-30T21:52:10", "heart_rate": uint8(152)},
			{"timestamp": "2021-12-30T21:52:12", "heart_rate": uint8(153)},
		}},
	}

	enrichLapHeartRates(out)

	assertEqual(t, out.Timeline.Laps[0]["start_heart_rate"], 0)
	assertEqual(t, out.Timeline.Laps[0]["end_heart_rate"], 0)
	assertEqual(t, out.Timeline.Laps[1]["start_heart_rate"], 0)
	assertEqual(t, out.Timeline.Laps[1]["end_heart_rate"], 0)
}

func TestMessageNameMatchesPythonUnknowns(t *testing.T) {
	assertEqual(t, messageName(typedef.MesgNumFileId), "file_id")
	assertEqual(t, messageName(typedef.MesgNum(216)), "unknown_216")
	assertEqual(t, messageName(typedef.MesgNum(9999)), "unknown_9999")
}

func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("got %#v (%T), want %#v (%T)", got, got, want, want)
	}
}
