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

package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/cobaltspeech/cubic-cli/internal/formatters/results/timeline"
	cubic "github.com/cobaltspeech/sdk-cubic/grpc/go-cubic"
	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
	pbduration "github.com/golang/protobuf/ptypes/duration"
	"github.com/spf13/cobra"
)

type inputs struct {
	uttID    string
	filepath string
}

type outputs struct {
	uttID    string
	segment  int
	response []*cubicpb.RecognitionResult
}

// Argument variables.
var model string
var inputFile string
var listFile bool
var resultsFile string
var outputFormat string
var nConcurrentRequests int
var audioChannels []int
var audioChannelsStereo bool
var maxAlternatives int

// Initialize flags.
func init() {
	transcribeCmd.PersistentFlags().StringVarP(&model, "model", "m", "1",
		"Selects which model ID to use for transcribing.\n"+
			"Must match a model listed from \"models\" subcommand.")

	transcribeCmd.Flags().BoolVarP(&listFile, "list-file", "l", false,
		"When true, the FILE_PATH is pointing to a file containing a list of \n"+
			"\"UtteranceID \\t path/to/audio.wav\", one entry per line.")

	transcribeCmd.Flags().StringVarP(&resultsFile, "outputFile", "o", "-",
		"File to send output to.  \"-\" indicates stdout.")

	transcribeCmd.Flags().StringVarP(&outputFormat, "outputFormat", "f", "timeline",
		"Format of output.  Can be [json,utterance-json,json-pretty,timeline].")

	transcribeCmd.Flags().IntSliceVarP(&audioChannels, "audioChannels", "c", []int{},
		"Audio channels to transcribe.  Defaults to mono.\n"+
			"  \"0\" for mono\n"+
			"  \"0,1\" for stereo\n"+
			"  \"0,2\" for first and third channels\n"+
			"Overrides --stereo if both are included.")

	transcribeCmd.Flags().BoolVar(&audioChannelsStereo, "stereo", false,
		"Sets --audioChannels \"0,1\" which transcribes both audio channels of a stereo file.\n"+
			"If --audioChannels is set, this flag is ignored.")

	transcribeCmd.Flags().IntVarP(&nConcurrentRequests, "workers", "n", 1,
		"Number of concurrent requests to send to cubicsvr.\n"+
			"Please note, while this value is defined client-side the performance \n"+
			"will be limited by the available computational ability of the server.  \n"+
			"If you are the only connection to an 8-core server, then \"-n 8\" is a \n"+
			"reasonable value.  A lower number is suggested if there are multiple \n"+
			"clients connecting to the same machine.")

	transcribeCmd.Flags().IntVarP(&maxAlternatives, "maxAlts", "a", 1,
		"Maximum number of alternatives to provide for each result, if the outputFormat includes alternatives (such as 'timeline').")
}

var longMsg = `
This command is used for transcribing audio files.
There are two modes: single file or list file.

    single file: transcribe FILE_PATH [flags]
      list file: transcribe FILE_PATH --list-file [flags]

In single file mode:
    The FILE_PATH should point to a single audio.wav file.

In list file mode:
    The FILE_PATH should point to a a file listing multiple audio files
    with the format "Utterance_ID \t FILE_PATH \n".
    Each entry should be on its own line.
    Utterance_IDs may not contain whitespace.

Audio files in the following formats are supported:
    WAV, FLAC, MP3, VOX, and RAW(PCM16SLE).

The file extension (wav, flac, mp3, vox, raw) will be used to determine which
 codec to use.  Use WAV or FLAC for best results.

Summary of "--outputFormat" options:
    * json           - json of map[uttID][]Results.
    * json-pretty    - json of map[uttID][]Results, prettified with newlines and indents.
    * utterance-json - "UttID_SegID \t json of a single Result", grouped by UttID.
    * timeline       - "start_time|channel_id|1best transcript", grouped by UttID.

See "transcribe --help" for details on the other flags.`

// Cmd is the command wrapping sub commands used to run audio file(s) through cubicsvr.
var transcribeCmd = &cobra.Command{
	Use: "transcribe FILE_PATH [flags]",
	Example: `
	# Single audio file
	transcribe FILE_PATH [flags]

	# List of audio files
	transcribe FILE_PATH --list-file [flags]`,
	Short:         "Command for transcribing audio file(s) through cubicsvr.",
	Long:          longMsg,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			cmd.Usage()
			fmt.Println()
			return fmt.Errorf("transcribe requires a FILE_PATH argument")
		}
		inputFile = args[0]

		// Check for overwritting an existing results file.
		if resultsFile != "-" {
			// Check to see if file exists
			if _, err := os.Stat(resultsFile); err == nil {
				// The file exists, since it didn't throw an error, so explain why we are quitting
				return fmt.Errorf("Aborting because --outputFile '%s' already exists", resultsFile)
			} else if !os.IsNotExist(err) {
				// We care about all errors, except the FileDoesntExist error.
				// That would indicate it is safe to procced with the program as normal
				return fmt.Errorf("Error while checking for existing --outputFile: %v", err)
			}
		}

		// Make sure outputFormat is a valid option.
		switch outputFormat {
		case "json": // Do nothing
		case "utterance-json": // Do nothing
		case "json-pretty": // Do nothing
		case "timeline": // Do nothing
		default:
			return fmt.Errorf("invalid option for outputFormat: '%v'", outputFormat)
		}

		// Set up audioChannels
		if len(audioChannels) == 0 { // Wasn't set
			if audioChannelsStereo {
				audioChannels = []int{0, 1} // Set to stereo
			}
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return transcribe()
	},
}

// transcribe is the main function.
// Flags have been previously verified in the cobra.Cmd.Args function above.
// It performs the following steps:
//   1. organizes the input file(s)
//   2. Starts up n [--workers] worker goroutines
//   3. passes all audiofiles to the workers
//   4. Collects the resulting transcription and outputs the results.
func transcribe() error {

	// Get output writter (file or stdout)
	outputWriter, err := getOutputWriter(resultsFile)
	if err != nil {
		return err
	}

	// Setup channels for communicating between the various goroutines
	fileChannel := make(chan inputs)
	resultsChannel := make(chan outputs)
	errChannel := make(chan error)

	// Set up a cubicsvr client
	client, err := createClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// Load the files and place them in a channel
	files, err := loadFiles(inputFile)
	if err != nil {
		return fmt.Errorf("Error loading file(s): %v", err)
	}
	verbosePrintf(os.Stdout, "Found %d files.\n", len(files))

	// Starts a goroutine that ads files to the channel and closes
	// it when there are no more files to add.
	feedInputFiles(fileChannel, files)

	// Starts multipe goroutines that each pull from the fileChannel,
	// send requests to cubic server, and then adds the results to the
	// results channel
	startWorkers(client, fileChannel, resultsChannel, errChannel)

	// Handle errors
	errCount := 0
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		verbosePrintf(os.Stdout, "Watching for errors during process.\n")
		for err := range errChannel {
			errCount++
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
		wg.Done()
	}()

	// Deal with the transcription results
	processResults(outputWriter, resultsChannel)

	wg.Wait()

	if errCount > 0 {
		fmt.Fprintf(os.Stderr, "-----------\nThere were a total of %d failed files.\n", errCount)
	}
	return nil
}

// Get a reference to outputfile/stdout, depending on the input of
func getOutputWriter(out string) (io.Writer, error) {
	if out == "-" {
		return os.Stdout, nil
	}

	file, err := os.Create(out)
	if err != nil {
		return nil, fmt.Errorf("Error opening output file: %v", err)
	}
	return file, nil
}

func loadFiles(path string) ([]inputs, error) {
	if listFile {
		return loadListFiles(path)
	}
	return loadSingleFile(path)
}

func loadSingleFile(path string) ([]inputs, error) {
	return []inputs{
		inputs{uttID: "utt_0", filepath: path},
	}, nil
}

func loadListFiles(path string) ([]inputs, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	folder := filepath.Dir(path)

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	utterances := make([]inputs, 0)
	for scanner.Scan() {
		txt := scanner.Text()

		// Allow empty lines in the file
		if txt == "" {
			continue
		}

		// Search for the first tab or space, split on that, and then trim up the filepath.
		whitespaceSet := "\t "
		var id, fpath string
		i := strings.IndexAny(txt, whitespaceSet)
		if i < 0 { // Didn't find a tab or space on this line.
			return nil, fmt.Errorf("Error parsing list file on line #%d, "+
				"format should be '[UttID]\\t[path/to/audio.wav]'.  "+
				"Line contents: '%s'", lineNumber+1, txt)
		}
		id = txt[:i]
		fpath = strings.Trim(txt[i:], whitespaceSet)

		// Convert relative paths to absolute paths
		if !filepath.IsAbs(fpath) {
			fpath, err = filepath.Abs(filepath.Join(folder, fpath))
			if err != nil {
				return nil, fmt.Errorf("Error converting audio file entry to absolute path\n"+
					"\tPath to listfile directory: '%s'\n"+
					"\tInput for entry (line #%d): '%s'\n"+
					"\tCombine path: '%s'\n"+
					"\tResulting AbsPath: '%s'\n"+
					"\tError: %v\n",
					folder, lineNumber+1, fpath,
					filepath.Join(folder, fpath),
					fpath, err)
			}
		}

		// Add the new entry to the list
		utterances = append(utterances, inputs{
			uttID:    id,
			filepath: fpath,
		})
		lineNumber++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return utterances, nil
}

func feedInputFiles(fileChannel chan<- inputs, files []inputs) {
	// Feed those files to the channel in a separate goroutine
	// so we can manage the results in the current goroutine.
	go func() {
		for _, f := range files {
			verbosePrintf(os.Stdout, "Feeding next file '%s'.\n", f.filepath)
			fileChannel <- f
		}
		verbosePrintf(os.Stdout, "Done feeding audio files.\n")
		close(fileChannel)
	}()
}

// Open the audio files and transcribe them, in parallel
func startWorkers(client *cubic.Client, fileChannel <-chan inputs,
	resultsChannel chan<- outputs, errChannel chan<- error) {

	wg := &sync.WaitGroup{}
	wg.Add(nConcurrentRequests)
	verbosePrintf(os.Stdout, "Starting '%d' workers.\n", nConcurrentRequests)
	for i := 0; i < nConcurrentRequests; i++ {
		go transcribeFiles(i, wg, client, fileChannel, resultsChannel, errChannel)
	}

	// close the results channel once all the workers have finished their steps
	go func() {
		wg.Wait()
		close(resultsChannel)
		close(errChannel)
		verbosePrintf(os.Stdout, "Finished getting results back from cubicsvr for all files.\n")
	}()
}

func transcribeFiles(workerID int, wg *sync.WaitGroup, client *cubic.Client,
	fileChannel <-chan inputs, resultsChannel chan<- outputs, errChannel chan<- error) {
	verbosePrintf(os.Stdout, "Worker %d starting\n", workerID)
	for input := range fileChannel {

		// Open the file
		audio, err := os.Open(input.filepath)
		if err != nil {
			errChannel <- fmt.Errorf(
				"Error: skipping Utterance '%s', couldn't open file '%s'",
				input.uttID, input.filepath)
			continue
		}

		var audioEncoding cubicpb.RecognitionConfig_Encoding
		ext := strings.ToLower(filepath.Ext(input.filepath))
		switch ext {
		case ".wav":
			audioEncoding = cubicpb.RecognitionConfig_WAV
		case ".flac":
			audioEncoding = cubicpb.RecognitionConfig_FLAC
		case ".mp3":
			audioEncoding = cubicpb.RecognitionConfig_MP3
		case ".vox":
			audioEncoding = cubicpb.RecognitionConfig_VOX8000
		case ".raw":
			audioEncoding = cubicpb.RecognitionConfig_RAW_LINEAR16
		default:
			errChannel <- fmt.Errorf("skipping utterance %q: unknown file extension %q", input.uttID, ext)
			continue

		}
		// Counter for segments
		segmentID := 0

		verbosePrintf(os.Stdout, "Worker%2d streaming Utterance '%s' (file '%s').\n",
			workerID, input.uttID, input.filepath)

		//Convert audioChannel from int to uint32
		var audioChannelsUint32 []uint32
		for _, c := range audioChannels {
			audioChannelsUint32 = append(audioChannelsUint32, uint32(c))
		}

		// Create and send the Streaming Recognize config
		err = client.StreamingRecognize(context.Background(),
			&cubicpb.RecognitionConfig{
				ModelId:               model,
				AudioEncoding:         audioEncoding,
				EnableWordTimeOffsets: true,
				EnableRawTranscript:   true,
				IdleTimeout:           &pbduration.Duration{Seconds: 30},
				AudioChannels:         audioChannelsUint32,
			},
			audio, // The file to send
			func(response *cubicpb.RecognitionResponse) { // The callback for results
				verbosePrintf(os.Stdout, "Worker%2d recieved result segment #%d for Utterance '%s'.\n", workerID, segmentID, input.uttID)
				resultsChannel <- outputs{
					uttID:    input.uttID,
					segment:  segmentID,
					response: response.Results, // TODO return the individual results, instead of slice of them.
				}
				segmentID++
			})
		if err != nil {
			verbosePrintf(os.Stderr, "Error streaming file: %v\n", err)
			errChannel <- simplifyGrpcErrors("error streaming the file", err)
		}
	}
	verbosePrintf(os.Stdout, "Worker %d done\n", workerID)
	wg.Done()
}

func processResults(outputWriter io.Writer, resultsChannel <-chan outputs) {
	// Keep a list of all non-partial results.  Cache them until all results have been recieved.
	finalResults := make(map[string][]*cubicpb.RecognitionResult)

	// Append every non-partial result to list.  Continues reading until the channel is closed.
	for result := range resultsChannel {
		for _, resp := range result.response {
			if !resp.IsPartial {
				finalResults[result.uttID] = append(finalResults[result.uttID], resp)
			}
		}
	}

	// Sort the uttIDs for consistancy between runs.
	var uttIDs []string
	for k := range finalResults {
		uttIDs = append(uttIDs, k)
	}
	sort.Strings(uttIDs)

	// Write formatted results to the outputWritter.
	switch outputFormat {
	case "utterance-json":
		for _, uttID := range uttIDs {
			segments, ok := finalResults[uttID]
			if !ok {
				fmt.Fprintf(os.Stderr, "Internal error: invalid uttID '%s'\n", uttID)
			} else {
				for nSegment, result := range segments {
					if str, err := json.Marshal(result); err != nil {
						fmt.Fprintf(os.Stderr, "[Error serializing]: %s_Segment%d\t%v\n", uttID, nSegment, err)
					} else {
						fmt.Fprintf(outputWriter, "%s_%d\t%s\n", uttID, nSegment, string(str))
					}
				}
			}
		}
	case "json":
		if str, err := json.Marshal(finalResults); err == nil {
			fmt.Fprint(outputWriter, string(str))
		} else {
			fmt.Fprintf(os.Stderr, "Error serializing results: %v\n", err)
		}
	case "json-pretty":
		if str, err := json.MarshalIndent(finalResults, "", "  "); err == nil {
			fmt.Fprint(outputWriter, string(str))
		} else {
			fmt.Fprintf(os.Stderr, "Error serializing results: %v\n", err)
		}
	case "timeline":
		cfg := timeline.Config{MaxAlternatives: maxAlternatives}
		formatter, err := cfg.CreateFormatter()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Internal error creating timeline formatter: %v", err)
			return
		}
		for _, uttID := range uttIDs {
			if results, ok := finalResults[uttID]; !ok {
				fmt.Fprintf(os.Stderr, "Internal error: invalid uttID '%s'\n", uttID)
			} else {
				output, err := formatter.Format(results)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Formatting results: %v", err)
					return
				}
				fmt.Fprintf(outputWriter, "Timeline for '%s':\n%s\n\n", uttID, output)
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "Internal error: invalid outputFormat '%s'\n", outputFormat)
	}

	if resultsFile != "-" {
		fmt.Printf("\nFinished writting results to file '%s'!\n\n", resultsFile)
	}
}
