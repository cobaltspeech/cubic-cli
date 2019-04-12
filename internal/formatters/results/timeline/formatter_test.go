// Copyright (2019) Cobalt Speech and Language Inc.  All rights reserved.

package timeline_test

import (
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

	want := "500|0|One0\n500|1|One1\n500|0|One0.1\n750|1|Two\n7500|0|Three"

	// Format the test RecognitionResult.
	got := timeline.Format(input)

	// Test for either of the two possible answers.
	if got != want {
		t.Errorf("Unexpected format recieved.\nGot:\n%s\n\nWant:\n\t%s\n\n", got, want)
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
			Transcript: transcript,
			Words: []*cubicpb.WordInfo{&cubicpb.WordInfo{
				StartTime: &duration.Duration{
					Seconds: int64(startTime / 1000),
					Nanos:   int32((startTime % 1000) * 1000000),
				},
			}},
		}},
	}
}
