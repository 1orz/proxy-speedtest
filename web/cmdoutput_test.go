package web

import (
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Date(2026, 7, 24, 15, 30, 5, 0, time.UTC) }

func TestParseOutputPlan(t *testing.T) {
	ts := "speedtest-20260724-153005"
	cases := []struct {
		name            string
		spec, file, pic string
		want            []OutputTarget
		wantErr         bool
	}{
		{"default empty -> json stdout", "", "", "", []OutputTarget{{"json", ""}}, false},
		{"single csv stdout", "csv", "", "", []OutputTarget{{"csv", ""}}, false},
		{"single csv to file", "csv", "r.csv", "", []OutputTarget{{"csv", "r.csv"}}, false},
		{"dedupe + order", "json,json,csv", "", "", []OutputTarget{{"json", ts + ".json"}, {"csv", ts + ".csv"}}, false},
		{"two data no file -> auto files", "json,csv", "", "", []OutputTarget{{"json", ts + ".json"}, {"csv", ts + ".csv"}}, false},
		{"first data uses file verbatim, extra auto", "json,csv", "out.json", "", []OutputTarget{{"json", "out.json"}, {"csv", ts + ".csv"}}, false},
		{"json + pic stdout + default png", "json,pic", "", "", []OutputTarget{{"json", ""}, {"pic", ts + ".png"}}, false},
		{"pic only default png", "pic", "", "", []OutputTarget{{"pic", ts + ".png"}}, false},
		{"pic only with output-pic", "pic", "", "p.png", []OutputTarget{{"pic", "p.png"}}, false},
		{"pic only, no pic path, uses output-file", "pic", "shot.png", "", []OutputTarget{{"pic", "shot.png"}}, false},
		{"none only -> empty", "none", "", "", []OutputTarget{}, false},
		{"none combined dropped", "json,none", "", "", []OutputTarget{{"json", ""}}, false},
		{"unknown -> error", "json,foo", "", "", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseOutputPlan(c.spec, c.file, c.pic, fixedNow())
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("len=%d want=%d (%v)", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("target[%d]=%+v want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}
