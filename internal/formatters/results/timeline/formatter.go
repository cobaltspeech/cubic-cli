// Copyright (2019) Cobalt Speech and Language Inc.  All rights reserved.

package timeline

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
	"github.com/golang/protobuf/ptypes/duration"
)

// Alternative is a subset of the fields in cubicpb.RecognitionAlternative
type Alternative struct {
	StartTime  int
	Confidence int
	Transcript string
}

// Result encapsulates the Alternatives for a specific endpointed utterance
type Result struct {
	ChannelID int
	Nbest     []Alternative
}

// Format generates a list of formatted utterances sorted by StartTime, smallest to largest.
//
// Some Promises:
// * The list will be sorted by starttime, smallest to largest.
// * If the startTime in milliseconds of two results are the same, they will be returned in the order the recognizer emitted them.
func Format(results []*cubicpb.RecognitionResult) string {

	// Populate list of Intermediate representation objects
	var entries []Result
	for _, r := range results {
		if r.IsPartial {
			continue
		}

		// Sometimes, there are entries that have an empty transcript as the most confident result, but may have other
		// transcripts at lower confidences.  For the purpose of the timeline, we prune those out.
		if hasEmptyTranscript(r) {
			continue
		}
		entry := Result{
			ChannelID: int(r.AudioChannel),
		}
		for _, alternative := range r.GetAlternatives() {

			entry.Nbest = append(entry.Nbest, Alternative{
				StartTime:  durToMs(alternative.StartTime),
				Confidence: int(alternative.Confidence * 1000),
				Transcript: alternative.Transcript,
			})
		}
		entries = append(entries, entry)
	}

	// Sort by startTime (results.Alternatives[0].Words[0].StartTime)
	// If start times are the same, maintain the original order.
	//
	// While CubicSvr guarentees that the of results entries are order
	// chronologicaly _per channel_, there is no such promise made about the
	// relationship between channels. Thus, the following sort is required.
	sort.Slice(entries, func(i, j int) bool {
		// Note: If the config didn't include {EnableWordTimeOffsets: true, EnableRawTranscript: true}
		// then they won't get timestamps, meaning start times are _all_ -1.
		// If that's the case, we can just maintain the same order they came in.
		// To do that, this function should return false.  `t1 < t2` still matches that pattern.
		return entries[i].Nbest[0].StartTime < entries[j].Nbest[0].StartTime
	})

	str, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error serializing results: %v\n", err)
	}

	return string(str)
}

func durToMs(d *duration.Duration) int {
	if d == nil {
		return -1
	}
	return int(d.Seconds*1000) + int(d.Nanos/1000/1000)
}

func hasEmptyTranscript(r *cubicpb.RecognitionResult) bool {
	return len(r.Alternatives) < 1 || r.Alternatives[0].Transcript == ""
}
