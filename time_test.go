// Copyright 2026 The naja-go Authors. All rights reserved.
// Copyright 2012 Alex Ogier. All rights reserved.
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpflag

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/naja-go/gpflag/pkg/types"
)

func setUpTimeVar(t *types.Time, formats []string) *FlagSet {
	f := NewFlagSet("test", ContinueOnError)
	f.NewVar(t, "time", types.Time{Time: t.Time, Formats: formats}, "Time")
	return f
}

func TestTime(t *testing.T) {
	testCases := []struct {
		input    string
		success  bool
		expected time.Time
	}{
		{"2022-01-01T01:01:01+00:00", true, time.Date(2022, 1, 1, 1, 1, 1, 0, time.UTC)},
		{" 2022-01-01T01:01:01+00:00", true, time.Date(2022, 1, 1, 1, 1, 1, 0, time.UTC)},
		{"2022-01-01T01:01:01+00:00 ", true, time.Date(2022, 1, 1, 1, 1, 1, 0, time.UTC)},
		{"2022-01-01T01:01:01+02:00", true, time.Date(2022, 1, 1, 1, 1, 1, 0, time.FixedZone("UTC+2", 2*60*60))},
		{"2022-01-01T01:01:01.01+02:00", true, time.Date(2022, 1, 1, 1, 1, 1, 10000000, time.FixedZone("UTC+2", 2*60*60))},
		{"Sat, 01 Jan 2022 01:01:01 +0000", true, time.Date(2022, 1, 1, 1, 1, 1, 0, time.UTC)},
		{"Sat, 01 Jan 2022 01:01:01 +0200", true, time.Date(2022, 1, 1, 1, 1, 1, 0, time.FixedZone("UTC+2", 2*60*60))},
		{"Sat, 01 Jan 2022 01:01:01 +0000", true, time.Date(2022, 1, 1, 1, 1, 1, 0, time.UTC)},
		{"", false, time.Time{}},
		{"not a date", false, time.Time{}},
		{"2022-01-01 01:01:01", false, time.Time{}},
		{"2022-01-01T01:01:01", false, time.Time{}},
		{"01 Jan 2022 01:01:01 +0000", false, time.Time{}},
		{"Sat, 01 Jan 2022 01:01:01", false, time.Time{}},
	}

	for i := range testCases {
		var timeVar types.Time
		formats := []string{time.RFC3339Nano, time.RFC1123Z}
		f := setUpTimeVar(&timeVar, formats)

		tc := &testCases[i]

		arg := fmt.Sprintf("--time=%s", tc.input)
		err := f.Parse([]string{arg})
		if err != nil && tc.success == true {
			t.Errorf("expected success, got %q", err)
			continue
		} else if err == nil && tc.success == false {
			t.Errorf("expected failure")
			continue
		} else if tc.success {
			timeResult, err := f.TryGet[types.Time]("time")
			if err != nil {
				t.Errorf("Got error trying to fetch the Time flag: %v", err)
			}
			if !timeResult.Equal(tc.expected) {
				t.Errorf("expected %q, got %q", tc.expected.Format(time.RFC3339Nano), timeVar.Format(time.RFC3339Nano))
			}
		}
	}
}

func usageForTimeFlagSet(t *testing.T, timeVar time.Time) string {
	t.Helper()
	formats := []string{time.RFC3339Nano, time.RFC1123Z}
	f := setUpTimeVar(&types.Time{Time: timeVar}, formats)
	if err := f.Parse([]string{}); err != nil {
		t.Fatalf("expected success, got %q", err)
	}
	return f.FlagUsages()
}

func TestTimeDefaultZero(t *testing.T) {
	usage := usageForTimeFlagSet(t, time.Time{})
	if strings.Contains(usage, "default") {
		t.Errorf("expected no default value in usage, got %q", usage)
	}
}

func TestTimeDefaultNonZero(t *testing.T) {
	timeVar := time.Date(2025, 1, 1, 1, 1, 1, 0, time.UTC)
	usage := usageForTimeFlagSet(t, timeVar)
	if !strings.Contains(usage, "default") || !strings.Contains(usage, "2025") {
		t.Errorf("expected default value in usage, got %q", usage)
	}
}
