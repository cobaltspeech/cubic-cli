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

package timeline

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
)

// Format generates a list of formatted utterances.
// The format of each utterance is "start_time_ms|audio_channel_id|1-best transcript".
//
// Some assumptions:
// * The config used in the original transcribe request included, so that timestamps will be available:
//   &cubicpb.RecognitionConfig{
//     EnableWordTimeOffsets: true,
//     EnableRawTranscript: true
//   }
//
// Some Promises:
// * The list will be sorted by starttime, smallest to largets.
// * No spaces will be present until after the second pipe ('|')
// * If the startTime in milliseconds is the same value, the order in which transcripts are.
// * If there is no value in the results.Alternatives[0].Words[0].StartTime, a time of -1 will be assumed.
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
		return entries[i].startTime < entries[j].startTime
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
		// TODO: This will show up as -1 in the final result, and show up at the top
		// of the transcription instead of somewhat inline like it _could_ have been.
		// Do we need to throw an error instead?  If they are _all_ -1, it's fine.
		// If one is an odd ball, then things look weird.
		return -1
	}
	d := r.Alternatives[0].Words[0].StartTime
	return int(d.Seconds*1000) + int(d.Nanos/1000/1000)
}

func hasEmptyTranscript(r *cubicpb.RecognitionResult) bool {
	return len(r.Alternatives) < 1 || r.Alternatives[0].Transcript == ""
}
