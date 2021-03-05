// Copyright (2021) Cobalt Speech and Language Inc.

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

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/cobaltspeech/cubic-cli/internal/ratelimit"
	"github.com/cobaltspeech/log"
	"github.com/cobaltspeech/log/pkg/level"
	cubic "github.com/cobaltspeech/sdk-cubic/grpc/go-cubic"
	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
	"github.com/go-audio/wav"
	pbduration "github.com/golang/protobuf/ptypes/duration"
	"github.com/spf13/cobra"
)

var rtfCmd = &cobra.Command{
	Use:           "rtf [--server address:port] [--insecure]",
	Short:         "Sends audio to cubicsvr, measures duration, calculates RTF.",
	SilenceErrors: true,
	Args: func(cmd *cobra.Command, args []string) error {
		if rtfInputFile == "" {
			return fmt.Errorf("--input must not be empty")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.NewLeveledLogger(log.WithFilterLevel(level.Verbosity(rtfVerbosity)), log.WithOutput(os.Stdout))
		err := runrtf(logger)
		if err != nil {
			logger.Error("msg", "failed to run", "error", err)
		}
	},
}

var (
	rtfModel               string
	rtfInputFile           string
	rtfNConcurrentRequests int
	rtfRealTime            bool
	rtfVerbosity           int
)

func init() {
	rtfCmd.Flags().StringVarP(&rtfModel, "model", "m", "1", ""+
		"Selects which model ID to use for transcribing.\n"+
		"Must match a model listed from \"models\" subcommand.")

	rtfCmd.Flags().StringVarP(&rtfInputFile, "input", "i", "", ""+
		"Path to audio file to be uploaded by each worker")

	rtfCmd.Flags().IntVarP(&rtfNConcurrentRequests, "workers", "n", 1, ""+
		"Number of concurrent requests to send to cubicsvr.\n"+
		"Please note, while this value is defined client-side the performance\n"+
		"will be limited by the available computational ability of the server.\n"+
		"If you are the only connection to an 8-core server, then \"-n 8\" is a\n"+
		"reasonable value.  A lower number is suggested if there are multiple\n"+
		"clients connecting to the same machine.")

	rtfCmd.Flags().BoolVar(&rtfRealTime, "real-time", false, "True puts in delays to "+
		"simulate streaming in real time.  False streams in batch mode (as fast "+
		"as it can).")

	rtfCmd.Flags().IntVarP(&rtfVerbosity, "verbosity", "v", 1, ""+
		"Log levels: [0=error, 1=info, 2=debug, 3=trace]")
}

func runrtf(logger log.Logger) error {
	// Set up a cubicsvr client
	client, err := createClient(cubicSvrAddress, insecure)
	if err != nil {
		return err
	}
	defer client.Close()

	wg := &sync.WaitGroup{}
	wg.Add(rtfNConcurrentRequests)
	globalStartTime := time.Now()

	// Start the workers
	var stats []rtfStats
	for i := 0; i < rtfNConcurrentRequests; i++ {
		go func(i int) {
			s, _ := streamFile(context.Background(), logger, client, i)
			stats = append(stats, s)
			wg.Done()
		}(i)
	}

	wg.Wait()
	globalEndTime := time.Now()

	globalDuration := globalEndTime.Sub(globalStartTime)
	globalStats := sum(stats)
	logger.Info("msg", "batch finished",
		"Global Wallclock Duration", globalDuration,
		"Threads", rtfNConcurrentRequests,
		"Files sent", globalStats.items,
		"RTF", globalStats.RTF(),
		"totalAudioDuration", globalStats.audioDuration,
		"TotalTranscribeDuration", globalStats.transcribeDuration,
		"AvgResultsLatency", time.Duration(float64(globalStats.resultsLatency)/float64(globalStats.items)))
	return nil
}

func streamFile(ctx context.Context, logger log.Logger, client *cubic.Client, workerID int) (rtfStats, error) {
	logger = log.With(logger, "workerID", workerID)

	cfg := &cubicpb.RecognitionConfig{
		ModelId:               rtfModel,
		AudioEncoding:         cubicpb.RecognitionConfig_WAV,
		IdleTimeout:           &pbduration.Duration{Seconds: 30},
		AudioChannels:         []uint32{0},
		EnableRawTranscript:   true,
		EnableWordConfidence:  true,
		EnableWordTimeOffsets: true,
	}

	// Calculate the wav file's duration
	audioDuration, audioBPS, err := wavStats(rtfInputFile)
	if err != nil {
		logger.Error("msg", "failed to calculate audio duration", "path", rtfInputFile, "error", err)
		return rtfStats{}, fmt.Errorf("failed to calculate audio duration")
	}

	// Open the file
	audio, err := os.Open(rtfInputFile)
	if err != nil {
		logger.Error("msg", "failed to read audio file", "path", rtfInputFile, "error", err)
		return rtfStats{}, fmt.Errorf("failed to read audio file")
	}
	defer audio.Close()

	// Wrap the audio file in a rate limited reader
	audioLimited := ratelimit.NewReader(ctx, audio)
	if rtfRealTime {
		audioLimited.SetRateLimit(float64(audioBPS))
	}

	startTime := time.Now()

	// Wrap the audio reader to get a timestamp when we are done reading/sending the audio file
	var streamDoneTime time.Time
	var bytesSent int64
	bytesTotal := getFileSize(rtfInputFile)
	progressCheckpoint := 0.0
	audioNotify := notifyReader{
		parent: audioLimited,
		readCallback: func(n int) {
			bytesSent += int64(n)

			// Report every 10% of the file
			progress := float64(bytesSent) / float64(bytesTotal)
			if progress > progressCheckpoint+0.1 {
				curDur := time.Now().Sub(startTime)
				logger.Debug("msg", "Progress Update", "progress", progress, "curDuration", curDur, "estTotalDuration", time.Duration(float64(curDur)/progress))

				// Update checkpoint to the closest 10% increment, rounding down (.57 -> .5, .23 -> .2)
				progressCheckpoint = float64(int(progress*10)) / 10
			}
		},
		eofCallback: func() {
			curDur := time.Now().Sub(startTime)
			logger.Debug("msg", "Progress Update", "progress", 1.0, "curDuration", curDur)
			streamDoneTime = time.Now()
		},
	}

	// TODO: consider wrapping the audio reader with a rate limiting reader for "real-time" streaming
	logger.Debug("msg", "Starting stream", "input", rtfInputFile)
	streamingErr := client.StreamingRecognize(ctx, cfg, audioNotify,
		func(response *cubicpb.RecognitionResponse) {
			logger.Trace("msg", "results came back")
		})
	endTime := time.Now()

	stats := rtfStats{
		items:              1,
		audioDuration:      audioDuration,
		transcribeDuration: endTime.Sub(startTime),
		resultsLatency:     endTime.Sub(streamDoneTime),
	}

	logger.Debug("msg", "worker finished",
		"RTF", stats.RTF(),
		"audioDuration", audioDuration,
		"transcribeDuration", stats.transcribeDuration,
		"resultsLatency", stats.resultsLatency,
		"err", streamingErr)

	return stats, streamingErr
}

type notifyReader struct {
	parent       io.Reader
	readCallback func(int)
	eofCallback  func()
}

func (tsr notifyReader) Read(buf []byte) (n int, err error) {
	n, err = tsr.parent.Read(buf)
	if err == io.EOF {
		if tsr.eofCallback != nil {
			tsr.eofCallback()
		}
	}
	if tsr.readCallback != nil {
		tsr.readCallback(n)
	}
	return n, err
}

func wavStats(path string) (time.Duration, int, error) {
	audio, err := os.Open(rtfInputFile)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read audio file: %v", err)
	}
	defer audio.Close()

	d := wav.NewDecoder(audio)

	dur, err := d.Duration()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to calculate audio duration: %v", err)
	}
	return dur, int(d.AvgBytesPerSec), nil
}

type rtfStats struct {
	items              int
	audioDuration      time.Duration
	transcribeDuration time.Duration
	resultsLatency     time.Duration
}

func sum(arr []rtfStats) rtfStats {
	out := rtfStats{items: len(arr)}
	for _, a := range arr {
		out.audioDuration += a.audioDuration
		out.transcribeDuration += a.transcribeDuration
		out.resultsLatency += a.resultsLatency
	}
	return out
}

func (s rtfStats) RTF() float64 {
	return float64(s.transcribeDuration) / float64(s.audioDuration)
}

func getFileSize(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		return -1
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return -1
	}

	return fi.Size()
}
