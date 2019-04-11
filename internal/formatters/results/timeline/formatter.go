// Copyright (2019) Cobalt Speech and Language Inc.  All rights reserved.

package timeline

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
)

// Format generates a list of formatted utterances.
// The format of a single utterance is "start_time_ms|audio_channel_id|1-best transcript"
// The list will be sorted by starttime, smallest to largets.
// No spaces will be present until after the second pipe ('|')
// If the startTime in milliseconds is the same value, the order in which transcripts are listed is undefined.
func Format(results []*cubicpb.RecognitionResult) string {
	// Intermediate representation
	type utterance struct {
		startTime  int
		channelID  int
		transcript string
	}

	// Populate list of Intermediate representation objects
	var entries []utterance
	for _, result := range results {
		if result.IsPartial {
			continue
		}

		// Sometimes, there are entries that have an empty transcript as the most confident result, but may have other
		// transcripts at lower confidences.  For the purpose of the timeline, we prune those out.
		if hasEmptyTranscript(result) {
			continue
		}

		entries = append(entries, utterance{
			startTime:  startTime(result),
			channelID:  int(result.AudioChannel),
			transcript: result.Alternatives[0].Transcript,
		})
	}

	// Sort by startTime (resp.Results.Alternatives[0].Words[0].StartTime)
	// If start times are the same, sort by channelID.
	// While CubicSvr guarentees that the of results entries are order
	// chronologicaly _per channel_, there is no such promise made about the
	// relationship between channels. Thus, the following sort is required.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].startTime != entries[j].startTime {
			return entries[i].startTime < entries[j].startTime
		}

		// Note: If the config didn't include {EnableWordTimeOffsets: true, EnableRawTranscript: true}
		// then they won't get timestamps, Meaning, start times are _all_ -1.
		// If that's the case, TODO(jhollowayj): do we want to skip sorting by channelID?
		if entries[i].startTime != -1 {
			return entries[i].channelID < entries[j].channelID
		}
		return false // Maintain the same order
	})

	// Convert each entry to the formatted string.
	var arr []string
	for _, e := range entries {
		arr = append(arr, fmt.Sprintf("%d|%d|%s", e.startTime, e.channelID, e.transcript))
	}

	return strings.Join(arr, "\n")
}

// Returns the start time in ms of the given RecognitionResult
func startTime(r *cubicpb.RecognitionResult) int {
	if len(r.Alternatives) == 0 || len(r.Alternatives[0].Words) == 0 {
		// TODO: This will show up as -1000 in the final result, and show up at the top of the transcription
		// Do we need to throw an error instead?
		return -1
	}
	d := r.Alternatives[0].Words[0].StartTime
	return int(d.Seconds*1000) + int(d.Nanos/1000/1000)
}

func hasEmptyTranscript(r *cubicpb.RecognitionResult) bool {
	return len(r.Alternatives) < 1 || r.Alternatives[0].Transcript == ""
}
