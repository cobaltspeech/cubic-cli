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
	"strings"
	"sync"

	"github.com/cobaltspeech/cubic-cli/internal/formatters/results/timeline"
	cubic "github.com/cobaltspeech/sdk-cubic/grpc/go-cubic"
	"github.com/cobaltspeech/sdk-cubic/grpc/go-cubic/cubicpb"
	pbduration "github.com/golang/protobuf/ptypes/duration"
	"github.com/spf13/cobra"
)

type inputs struct {
	uttID      string
	filepath   string
	outputPath string
}

type outputs struct {
	UttID        string
	Responses    []*cubicpb.RecognitionResult
	outputWriter io.WriteCloser
}

// Argument variables.
var model string
var inputFile string
var listFile bool
var resultsPath string
var outputFormat string
var nConcurrentRequests int
var audioChannels []int
var audioChannelsStereo bool
var enableRawTranscript bool
var maxAlternatives int

// Initialize flags.
func init() {
	transcribeCmd.PersistentFlags().StringVarP(&model, "model", "m", "1", ""+
		"Selects which model ID to use for transcribing.\n"+
		"Must match a model listed from \"models\" subcommand.")

	transcribeCmd.Flags().BoolVarP(&listFile, "list-file", "l", false, ""+
		"When true, the FILE_PATH is pointing to a file containing a list of \n"+
		"  \"UtteranceID \\t path/to/audio.wav\", one entry per line.")

	transcribeCmd.Flags().StringVarP(&resultsPath, "output", "o", "-", ""+
		"Path to where the results should be written to.\n"+
		"This path should be a directory.\n"+
		"In --list-file mode, each file processed will have a separate output file, \n"+
		"  using the utteranceID as the filename with a \".txt\" extention.\n"+
		"\"-\" indicates stdout in either case.  In --list-file mode, each entry will be \n"+
		"  prefaced by the utterance ID and have an extra newline seperating it from the next.")

	transcribeCmd.Flags().StringVarP(&outputFormat, "outputFormat", "f", "timeline",
		"Format of output.  Can be [json,json-pretty,timeline,utterance-json,stream].")

	transcribeCmd.Flags().IntSliceVarP(&audioChannels, "audioChannels", "c", []int{}, ""+
		"Audio channels to transcribe.  Defaults to mono.\n"+
		"  \"0\" for mono\n"+
		"  \"0,1\" for stereo\n"+
		"  \"0,2\" for first and third channels\n"+
		"Overrides --stereo if both are included.")

	transcribeCmd.Flags().BoolVar(&audioChannelsStereo, "stereo", false, ""+
		"Sets --audioChannels \"0,1\" to transcribe both audio channels of a stereo file.\n"+
		"If --audioChannels is set, this flag is ignored.")

	transcribeCmd.Flags().BoolVar(&enableRawTranscript, "enableRawTranscript", false, ""+
		"Sets the EnableRawTranscript field of the RecognizeRequest to true.")

	transcribeCmd.Flags().IntVarP(&nConcurrentRequests, "workers", "n", 1, ""+
		"Number of concurrent requests to send to cubicsvr.\n"+
		"Please note, while this value is defined client-side the performance\n"+
		"will be limited by the available computational ability of the server.\n"+
		"If you are the only connection to an 8-core server, then \"-n 8\" is a\n"+
		"reasonable value.  A lower number is suggested if there are multiple\n"+
		"clients connecting to the same machine.")

	transcribeCmd.Flags().IntVarP(&maxAlternatives, "fmt.timeline.maxAlts", "a", 1, ""+
		"Maximum number of alternatives to provide for each result, if the outputFormat\n"+
		"includes alternatives (such as 'timeline').")

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
    * timeline       - "start_time|channel_id|1best transcript", grouped by UttID.
    * utterance-json - "UttID_SegID \t json of a single Result", grouped by UttID.
    * stream         - Outputs the 1-best transcript as soon as the results come back.
                       No ordering guarentees are offered.  No headers/UttIDs are provided.
                       Partial results are excluded.

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

		// Make sure outputFormat is a valid option.
		switch outputFormat {
		case "json": // Do nothing
		case "utterance-json": // Do nothing
		case "json-pretty": // Do nothing
		case "timeline": // Do nothing
		case "stream": // Do nothing
		default:
			return fmt.Errorf("invalid option for outputFormat: '%v'", outputFormat)
		}

		// utterance-json must print to stdout
		if outputFormat == "utterance-json" && resultsPath != "-" {
			return fmt.Errorf("Aborting: --outputFormat utterance-json can only write to stdout")
		}
		// stream must print to stdout
		if outputFormat == "stream" && resultsPath != "-" {
			return fmt.Errorf("Aborting: --outputFormat stream can only write to stdout")
		}

		// Check for overwritting an existing results file.
		if resultsPath != "-" {
			isFolder, exists, err := statsPath(resultsPath)
			if err != nil {
				return fmt.Errorf("Error accessing --output '%s': %v", resultsPath, err)
			}

			// --output needs to be a valid (existing) folder.
			if !exists {
				return fmt.Errorf("Aborting: --output must be an existing directory: %v", resultsPath)
			}
			if !isFolder {
				return fmt.Errorf("Aborting: --output must be a directory: %v", resultsPath)
			}

			// Checking for individual files happens later
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

func statsPath(path string) (isFolder, exists bool, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// See if it should be a file or folder
			isFolder := strings.HasSuffix(path, string(filepath.Separator))
			return isFolder, false, nil
		}
		return false, false, err
	}
	return fi.Mode().IsDir(), true, nil
}

// transcribe is the main function.
// Flags have been previously verified in the cobra.Cmd.Args function above.
// It performs the following steps:
//   1. organizes the input file(s)
//   2. Starts up n [--workers] worker goroutines
//   3. passes all audiofiles to the workers
//   4. Collects the resulting transcription and outputs the results.
func transcribe() error {
	// Setup channels for communicating between the various goroutines
	fileChannel := make(chan inputs)
	fileResultsChannel := make(chan outputs)
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
	startWorkers(client, fileChannel, fileResultsChannel, errChannel)

	// Handle errors
	errorWaitGroup := &sync.WaitGroup{} // Wait group to let the main process know we are done reporting errors
	errorWaitGroup.Add(1)
	go func() {
		errCount := 0
		verbosePrintf(os.Stdout, "Watching for errors during process.\n")
		for err := range errChannel { // Block until the channel closes
			errCount++
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
		if errCount > 0 {
			fmt.Fprintf(os.Stderr, "-----------\nThere were a total of %d failed files.\n", errCount)
		}
		errorWaitGroup.Done()
	}()

	// Deal with the transcription results
	// Note, these processResults* functions will block until all transcription results have been recieved.
	nFilesWritten := 0
	switch outputFormat {
	case "utterance-json":
		nFilesWritten = processResultsUtteranceJSON(fileResultsChannel)
	case "json":
		nFilesWritten = processResultsJSON(fileResultsChannel, false)
	case "json-pretty":
		nFilesWritten = processResultsJSON(fileResultsChannel, true)
	case "timeline":
		nFilesWritten = processResultsTimeline(fileResultsChannel)
	case "stream":
		// Do nothing, the results are written directly to stdout as they are recieved.
	default:
		fmt.Fprintf(os.Stderr, "Internal error: invalid outputFormat '%s'\n", outputFormat)
	}

	if resultsPath != "-" && nFilesWritten > 0 {
		if listFile {
			fmt.Printf("Finished writting %d results to folder '%s'!\n", nFilesWritten, resultsPath)
		} else {
			fmt.Printf("Finished writting results to file '%s'!\n", resultsPath)
		}
	}

	errorWaitGroup.Wait() // Wait for all errors to finish

	return nil
}

// Get a reference to output/stdout, depending on the input of
func getOutputWriter(path string) (io.WriteCloser, error) {
	if path == "-" {
		return os.Stdout, nil
	}

	// Create the file
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to create output file: %v", err)
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
	// Generate an output filepath for the input file
	var outputPath = "-"
	if resultsPath != "-" {
		outputPath = filepath.Join(resultsPath, filepath.Base(path)+".txt")
		if isFolder, exists, err := statsPath(outputPath); err != nil {
			return nil, fmt.Errorf("Failed setting up a results file: %v", err)
		} else if !isFolder && exists {
			return nil, fmt.Errorf("Error: Results file '%s' already exists", outputPath)
		}
	}

	return []inputs{
		inputs{
			uttID:      filepath.Base(path),
			filepath:   path,
			outputPath: outputPath, // Already checked for existing output file.
		},
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

		// Generate an output file for each input file
		var outputPath = "-"
		if resultsPath != "-" {
			outputPath = filepath.Join(resultsPath, id+".txt")
			if isFolder, exists, err := statsPath(outputPath); err != nil {
				return nil, fmt.Errorf("Failed setting up a results file: %v", err)
			} else if !isFolder && exists {
				return nil, fmt.Errorf("Error: Results file '%s' already exists", outputPath)
			}
		}

		// Add the new entry to the list
		utterances = append(utterances, inputs{
			uttID:      id,
			filepath:   fpath,
			outputPath: outputPath,
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
	fileResultsChannel chan<- outputs, errChannel chan<- error) {

	wg := &sync.WaitGroup{}
	wg.Add(nConcurrentRequests)
	verbosePrintf(os.Stdout, "Starting '%d' workers.\n", nConcurrentRequests)
	for i := 0; i < nConcurrentRequests; i++ {
		go transcribeFiles(i, wg, client, fileChannel, fileResultsChannel, errChannel)
	}

	// close the results channel once all the workers have finished their steps
	go func() {
		wg.Wait()
		close(fileResultsChannel)
		close(errChannel)
		verbosePrintf(os.Stdout, "Finished getting results back from cubicsvr for all files.\n")
	}()
}

func transcribeFiles(workerID int, wg *sync.WaitGroup, client *cubic.Client,
	fileChannel <-chan inputs, fileResultsChannel chan<- outputs, errChannel chan<- error) {
	verbosePrintf(os.Stdout, "Worker %d starting\n", workerID)
	for input := range fileChannel {

		// Open the file
		audio, err := os.Open(input.filepath)
		if err != nil {
			errChannel <- fmt.Errorf(
				"Error: skipping Utterance '%s', couldn't open audio file '%s'",
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
			//TODO(julie): Should require them to specify the audio encoding
			// for headerless formats, either as part of the call or in a config file
			audioEncoding = cubicpb.RecognitionConfig_ULAW8000
		case ".raw":
			audioEncoding = cubicpb.RecognitionConfig_RAW_LINEAR16
		default:
			errChannel <- fmt.Errorf("skipping utterance '%s': unknown file extension %s", input.uttID, ext)
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

		cfg := &cubicpb.RecognitionConfig{
			ModelId:       model,
			AudioEncoding: audioEncoding,
			IdleTimeout:   &pbduration.Duration{Seconds: 30},
			AudioChannels: audioChannelsUint32,
		}

		if enableRawTranscript {
			cfg.EnableRawTranscript = true
		}

		if outputFormat == "timeline" {
			cfg.EnableWordConfidence = true
			cfg.EnableWordTimeOffsets = true
		}

		// create buffer for file's results
		var fileResponses []*cubicpb.RecognitionResult
		// Create and send the Streaming Recognize config
		err = client.StreamingRecognize(context.Background(),
			cfg,
			audio, // The audio file to send
			func(response *cubicpb.RecognitionResponse) { // The callback for results
				verbosePrintf(os.Stdout, "Worker%2d recieved result segment #%d for Utterance '%s'.\n", workerID, segmentID, input.uttID)
				if outputFormat == "stream" {
					// Print the response to stdout
					for _, r := range response.Results {
						if !r.IsPartial && len(r.Alternatives) > 0 {
							fmt.Fprintln(os.Stdout, r.Alternatives[0].Transcript)
						}
					}
				} else {
					fileResponses = append(fileResponses, response.Results...)
				}
				segmentID++
			})

		if err != nil {
			verbosePrintf(os.Stderr, "Error streaming file: %v\n", err)
			errChannel <- simplifyGrpcErrors("error streaming the file", err)
			continue
		}

		// Set up output file writer
		var outputWriter io.WriteCloser
		if outputFormat != "utterance-json" && outputFormat != "stream" { // These two force stdout, so skip this step for them.
			var err error
			outputWriter, err = getOutputWriter(input.outputPath)
			if err != nil {
				errChannel <- err
				continue
			}
		}

		if outputFormat != "stream" {
			fileResultsChannel <- outputs{
				UttID:        input.uttID,
				Responses:    fileResponses,
				outputWriter: outputWriter,
			}
		}
	}
	verbosePrintf(os.Stdout, "Worker %d done\n", workerID)
	wg.Done()
}

// processResultsUtteranceJSON returns a line for each entry in the format of:
//     {utteranceID}_{segment#} \t {json_serialization_of_results}
// --output must be "-" (stdout).
// The order of segments for a given utterance will be chronological.
// There is no guarantee for the order of utteranceIDs, as results are printed as soon as the file is completed.
func processResultsUtteranceJSON(fileResultsChannel <-chan outputs) int {
	// Write each file's results to the file
	for fileResults := range fileResultsChannel {
		uttID := fileResults.UttID
		response := fileResults.Responses
		for nSegment, result := range response {
			if str, err := json.Marshal(result); err != nil {
				fmt.Fprintf(os.Stderr, "[Error serializing]: %s_Segment%d\t%v\n", uttID, nSegment, err)
			} else {
				fmt.Fprintf(os.Stdout, "%s_%d\t%s\n", uttID, nSegment, string(str))
			}
		}
	}
	return 1 // This processResults function only writes to one file.
}

func processResultsJSON(fileResultsChannel <-chan outputs, pretty bool) int {
	count := 0
	for fileResults := range fileResultsChannel {
		count++ // Increment file count

		var bytes []byte
		var err error

		// Serialize the results
		if fileResults.outputWriter == os.Stdout { // Include UttID with results.
			if pretty {
				bytes, err = json.MarshalIndent(fileResults, "", "  ")
			} else {
				bytes, err = json.Marshal(fileResults)
			}
		} else { // Only include the results, exclude the UttID
			if pretty {
				bytes, err = json.MarshalIndent(fileResults.Responses, "", "  ")
			} else {
				bytes, err = json.Marshal(fileResults.Responses)
			}
		}

		// Print results to outputWriter
		if err == nil {
			fmt.Fprint(fileResults.outputWriter, string(bytes))
			if fileResults.outputWriter == os.Stdout {
				fmt.Printf("\n") //Add a new line for each entry
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error serializing results: %v\n", err)
		}

		// Close files, but not stdout
		if fileResults.outputWriter != os.Stdout {
			fileResults.outputWriter.Close()
		}
	}
	return count
}

func processResultsTimeline(fileResultsChannel <-chan outputs) int {
	count := 0

	// Create formatter
	cfg := timeline.Config{MaxAlternatives: maxAlternatives}
	formatter, err := cfg.CreateFormatter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Internal error creating timeline formatter: %v", err)
		return 0
	}

	// Continues reading until the fileResultsChannel is closed
	for fileResults := range fileResultsChannel {
		count++ // Increment file count

		// Format the results
		output, err := formatter.Format(fileResults.Responses)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while formatting results: %v", err)
			continue
		}

		// Format and print the results to the outputWriter
		if resultsPath == "-" { //Add a header to each entry
			fmt.Fprintf(fileResults.outputWriter, "Timeline for '%s':\n%s\n\n", fileResults.UttID, output)
		} else { // When writing to a file, don't include the same header
			fmt.Fprintf(fileResults.outputWriter, "%s", output)
		}

		if fileResults.outputWriter != os.Stdout {
			fileResults.outputWriter.Close() // Close file.
		}
	}
	return count
}
