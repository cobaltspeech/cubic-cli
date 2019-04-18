// Copyright (2019) Cobalt Speech and Language Inc.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package timeline_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/cobaltspeech/cubic-cli/internal/formatters/results/timeline"
	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
	duration "github.com/golang/protobuf/ptypes/duration"
)

// TestExxonFormatter tests that the entries are sorted by timestamp
func TestExxonFormatter_Order(t *testing.T) {
	// Create a test case
	// Note: This test case has two entries with the same timestamp, it should sort by channelID
	input := []*cubicpb.RecognitionResult{
		newResult("Three", 7500, 0, false),
		newResult("Two", 750, 1, false),
		newResult("One0", 500, 0, false),
		newResult("One1", 500, 1, false),
		newResult("One0.1", 500, 0, false),
		newResult("partials are ignored", 1500, 0, true), // Shouldn't show up as a partial result.
		newResult("", 1500, 0, false),                    // Shouldn't show up with an empty transcript.
	}
	want := []timeline.Result{
		{
			ChannelID: 0,
			Nbest: []timeline.Alternative{
				{
					StartTime:  500,
					Confidence: 0.6,
					Transcript: "One0",
				},
			},
		},
		{
			ChannelID: 1,
			Nbest: []timeline.Alternative{
				{
					StartTime:  500,
					Confidence: 0.6,
					Transcript: "One1",
				},
			},
		},
		{
			ChannelID: 0,
			Nbest: []timeline.Alternative{
				{
					StartTime:  500,
					Confidence: 0.6,
					Transcript: "One0.1",
				},
			},
		},
		{
			ChannelID: 1,
			Nbest: []timeline.Alternative{
				{
					StartTime:  750,
					Confidence: 0.6,
					Transcript: "Two",
				},
			},
		},
		{
			ChannelID: 0,
			Nbest: []timeline.Alternative{
				{
					StartTime:  7500,
					Confidence: 0.6,
					Transcript: "Three",
				},
			},
		},
	}

	cfg := timeline.Config{
		MaxAlternatives: 1,
	}

	formatter, err := cfg.CreateFormatter()
	if err != nil {
		t.Errorf("Unexpected error creating formatter %s", err)
	}

	// Format the test RecognitionResult.
	str, err := formatter.Format(input)
	if err != nil {
		t.Errorf("Unexpected error calling Format %s", err)
	}

	var got []timeline.Result
	if err := json.Unmarshal([]byte(str), &got); err != nil {
		t.Errorf("Error unmarshalling formatted result: %s", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Unexpected output.\nGot:\n%v\n\nWant:\n%v\n\n", got, want)
	}
}

// newResult builds a partially populated RecognitionResult with only the three relevant fields being populated.
// There is only one alternative, with only one wordInfo.
// It is assumed that this is valid enough for testing the formatting and sorting features of this formatter.
func newResult(transcript string, startTime int, channelId int, partial bool) *cubicpb.RecognitionResult {
	return &cubicpb.RecognitionResult{
		AudioChannel: uint32(channelId),
		IsPartial:    partial,
		Alternatives: []*cubicpb.RecognitionAlternative{&cubicpb.RecognitionAlternative{
			Confidence: 0.6,
			Transcript: transcript,
			StartTime: &duration.Duration{
				Seconds: int64(startTime / 1000),
				Nanos:   int32((startTime % 1000) * 1000000),
			},
			Words: []*cubicpb.WordInfo{&cubicpb.WordInfo{
				Word:       transcript,
				Confidence: 0.6,
				StartTime: &duration.Duration{
					Seconds: int64(startTime / 1000),
					Nanos:   int32((startTime % 1000) * 1000000),
				},
			}},
		}},
	}
}
