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
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
	"github.com/golang/protobuf/ptypes/duration"
)

// Alternative is a subset of the fields in cubicpb.RecognitionAlternative
type Alternative struct {
	StartTime  int64               `json:"start_time"`
	Duration   int64               `json:"duration"`
	Confidence float64             `json:"confidence"`
	Transcript string              `json:"transcript"`
	Words      []*cubicpb.WordInfo `json:"-"`
}

func marshal(v interface{}) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("error serializing results: %v", err)
	}
	return buf, nil
}

// MarshalJSON hides the unfriendly JSON output for duration.Duration
// The pattern of aliasing the type we want to shadow is from the
// blog post http://choly.ca/post/go-json-marshalling/
func (a Alternative) MarshalJSON() ([]byte, error) {
	// create aliases so we have all the same fields but none of the methods and don't inherit the original type's MarshalJSON
	type Alias Alternative
	type WordAlias cubicpb.WordInfo
	type W struct {
		StartTime int64 `json:"start_time"`
		Duration  int64 `json:"duration"`
		*WordAlias
	}
	out := &struct {
		Words []*W `json:"words,omitempty"`
		Alias
	}{
		Alias: (Alias)(a),
	}

	for _, word := range a.Words {
		out.Words = append(out.Words, &W{
			StartTime: durToMs(word.StartTime),
			Duration:  durToMs(word.Duration),
			WordAlias: (*WordAlias)(word),
		})
	}
	buf, err := marshal(out)
	if err != nil {
		fmt.Println("Error in marshalJSON", err)
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

// Result encapsulates the Alternatives for a specific endpointed utterance
type Result struct {
	ChannelID int           `json:"channel_id"`
	Nbest     []Alternative `json:"nbest"`
}

// ResultFormatter formats a slice of RecognitionResults, e.g. an entire file's worth
type ResultFormatter interface {
	Format(results []*cubicpb.RecognitionResult) (string, error)
}

// Config holds the various options that can be used for this Formatter.
type Config struct {
	MaxAlternatives int
}

// CreateFormatter returns a new Formatter instance with the given configurations
func (cfg Config) CreateFormatter() (ResultFormatter, error) {
	if err := verifyCfg(cfg); err != nil {
		return nil, err
	}
	return Formatter{Cfg: cfg}, nil
}

func verifyCfg(cfg Config) error {
	if cfg.MaxAlternatives < 1 {
		return fmt.Errorf("Number of alternatives must be 1 or more")
	}

	return nil
}

// Formatter can be configured to limit the number of alternatives for each result
type Formatter struct {
	Cfg Config
}

// Format generates a list of formatted utterances sorted by StartTime, smallest to largest.
//
// Some Promises:
// * The list will be sorted by starttime, smallest to largest.
// * If the startTime in milliseconds of two results are the same, they will be returned in the order the recognizer emitted them.
func (f Formatter) Format(results []*cubicpb.RecognitionResult) (string, error) {

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
		for i, alternative := range r.GetAlternatives() {
			if i >= f.Cfg.MaxAlternatives {
				break
			}

			entry.Nbest = append(entry.Nbest, Alternative{
				StartTime:  durToMs(alternative.StartTime),
				Duration:   durToMs(alternative.Duration),
				Confidence: alternative.Confidence,
				Transcript: alternative.Transcript,
				Words:      alternative.Words,
			})
		}
		entries = append(entries, entry)
	}

	// Sort by startTime (results.Alternatives[0].StartTime)
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

	buf, err := marshal(&entries)
	if err != nil {
		return "", fmt.Errorf("error serializing results: %v", err)
	}
	return buf.String(), nil
}

func durToMs(d *duration.Duration) int64 {
	if d == nil {
		return -1
	}
	return int64(d.Seconds*1000) + int64(d.Nanos/1000/1000)
}

func hasEmptyTranscript(r *cubicpb.RecognitionResult) bool {
	return len(r.Alternatives) < 1 || r.Alternatives[0].Transcript == ""
}
