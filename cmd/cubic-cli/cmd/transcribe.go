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
	uttID        string
	filepath     string
	outputWriter io.Writer // gets passed to the outputs struct's outputWriter
}

type outputs struct {
	uttID        string
	responses    []responseResults
	outputWriter io.Writer
}
type responseResults []*cubicpb.RecognitionResult

// Argument variables.
var model string
var inputFile string
var listFile bool
var resultsPath string
var outputFormat string
var nConcurrentRequests int
var audioChannels []int
var audioChannelsStereo bool
var maxAlternatives int

// Initialize flags.
func init() {
	transcribeCmd.PersistentFlags().StringVarP(&model, "model", "m", "1", ""+
		"Selects which model ID to use for transcribing.\n"+
		"Must match a model listed from \"models\" subcommand.")

	transcribeCmd.Flags().BoolVarP(&listFile, "list-file", "l", false, ""+
		"When true, the FILE_PATH is pointing to a file containing a list of \n"+
		"\"UtteranceID \\t path/to/audio.wav\", one entry per line.")

	transcribeCmd.Flags().StringVarP(&resultsPath, "output", "o", "-", ""+
		"Path to where the results should be written to.\n"+
		"In single file mode, this path should be a file.\n"+
		"In list file mode, this path should be a directory.  Each file processed will\n"+
		"  have a separate output file, using the imput file's name, with a .txt extention.\n"+
		"\"-\" indicates stdout in either case.  In list file mode, each entry will be \n"+
		"  prefaced by the utterance ID and have an extra newline seperating it from the next. ")

	transcribeCmd.Flags().StringVarP(&outputFormat, "outputFormat", "f", "timeline",
		"Format of output.  Can be [json,utterance-json,json-pretty,timeline].")

	transcribeCmd.Flags().IntSliceVarP(&audioChannels, "audioChannels", "c", []int{}, ""+
		"Audio channels to transcribe.  Defaults to mono.\n"+
		"  \"0\" for mono\n"+
		"  \"0,1\" for stereo\n"+
		"  \"0,2\" for first and third channels\n"+
		"Overrides --stereo if both are included.")

	transcribeCmd.Flags().BoolVar(&audioChannelsStereo, "stereo", false, ""+
		"Sets --audioChannels \"0,1\" which transcribes both audio channels of a stereo file.\n"+
		"If --audioChannels is set, this flag is ignored.")

	transcribeCmd.Flags().IntVarP(&nConcurrentRequests, "workers", "n", 1, ""+
		"Number of concurrent requests to send to cubicsvr.\n"+
		"Please note, while this value is defined client-side the performance \n"+
		"will be limited by the available computational ability of the server.  \n"+
		"If you are the only connection to an 8-core server, then \"-n 8\" is a \n"+
		"reasonable value.  A lower number is suggested if there are multiple \n"+
		"clients connecting to the same machine.")

	transcribeCmd.Flags().IntVarP(&maxAlternatives, "fmt.timeline.maxAlts", "a", 1,
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
		if resultsPath != "-" {
			switch {
			case listFile:
				// In list file mode, we want either "-" or a valid folder.
				fi, err := os.Stat(resultsPath)
				switch {
				case err != nil:
					return fmt.Errorf("Aborting: failed getting stats about --output: %v", err)
				case !fi.Mode().IsDir():
					return fmt.Errorf("Aborting because --output '%s' is not a directory", resultsPath)
					// Note: Checking for existing `resultsPath/utteranceID_results.txt` files occures later
					//       in the loadListFiles() function, once we read the list file contents.
				}

			case !listFile:
				// In single file mode, we want either "-" or a non-existing file.
				// Check to see if file exists
				fi, err := os.Stat(resultsPath)
				switch {
				case err != nil:
					if !os.IsNotExist(err) {
						// We care about all errors, except the FileDoesntExist error.
						// That would indicate it is safe to procced with the program as normal
						return fmt.Errorf("Error while checking for existing --output: %v", err)
					}
				case fi.IsDir():
					return fmt.Errorf("Aborting because --output '%s' is a directory, not a file", resultsPath)
				case err == nil:
					// Else, the file exists, since it didn't throw an error, so explain why we are quitting
					return fmt.Errorf("Aborting because --output '%s' already exists", resultsPath)
				}
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
	switch outputFormat {
	case "utterance-json":
		processResultsUtteranceJSON(fileResultsChannel)
	case "json":
		processResultsJSON(fileResultsChannel, false)
	case "json-pretty":
		processResultsJSON(fileResultsChannel, true)
	case "timeline":
		processResultsTimeline(fileResultsChannel)
	default:
		fmt.Fprintf(os.Stderr, "Internal error: invalid outputFormat '%s'\n", outputFormat)
	}

	if resultsPath != "-" {
		fmt.Printf("\nFinished writting results to file '%s'!\n\n", resultsPath)
	}

	errorWaitGroup.Wait() // Wait for all errors to finish

	return nil
}

// Get a reference to output/stdout, depending on the input of
func getOutputWriter(out string) (io.Writer, error) {
	if out == "-" {
		return os.Stdout, nil
	}

	// Check for existing file
	if _, err := os.Stat(out); err == nil {
		// The file exists, since it didn't throw an error, so explain why we are quitting
		return nil, fmt.Errorf("file '%s' already exists", out)
	} else if !os.IsNotExist(err) {
		// We care about all errors, except the FileDoesntExist error.
		// That would indicate it is safe to procced with the program as normal
		return nil, fmt.Errorf("error while checking for existing file '%s': %v", out, err)
	}

	// Create the file
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
	outputWriter, err := getOutputWriter(resultsPath)
	if err != nil {
		return nil, fmt.Errorf("Failed opening output file: %v", err)
	}
	return []inputs{
		inputs{
			uttID:        "utt_0",
			filepath:     path,
			outputWriter: outputWriter,
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

		// Generate an outputWriter for each file
		var outputWriter io.Writer
		outputWriter = os.Stdout
		if resultsPath != "-" {
			var err error
			if outputWriter, err = getOutputWriter(filepath.Join(resultsPath, id+"_results.txt")); err != nil {
				return nil, fmt.Errorf("Failed setting up results file: %v", err)
			}
		}

		// Add the new entry to the list
		utterances = append(utterances, inputs{
			uttID:        id,
			filepath:     fpath,
			outputWriter: outputWriter,
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

		cfg := &cubicpb.RecognitionConfig{
			ModelId:       model,
			AudioEncoding: audioEncoding,
			IdleTimeout:   &pbduration.Duration{Seconds: 30},
			AudioChannels: audioChannelsUint32,
		}

		if outputFormat == "timeline" {
			cfg.EnableWordConfidence = true
			cfg.EnableWordTimeOffsets = true

		}

		// create buffer for file's results
		var fileResponses []responseResults
		// Create and send the Streaming Recognize config
		err = client.StreamingRecognize(context.Background(),
			cfg,
			audio, // The audio file to send
			func(response *cubicpb.RecognitionResponse) { // The callback for results
				verbosePrintf(os.Stdout, "Worker%2d recieved result segment #%d for Utterance '%s'.\n", workerID, segmentID, input.uttID)
				fileResponses = append(fileResponses, response.Results)
				segmentID++
			})

		if err != nil {
			verbosePrintf(os.Stderr, "Error streaming file: %v\n", err)
			errChannel <- simplifyGrpcErrors("error streaming the file", err)
			continue
		}

		if input.outputWriter == nil {
			fmt.Printf("Found another nil outputWriter\n")
		}

		fileResultsChannel <- outputs{
			uttID:        input.uttID,
			responses:    fileResponses,
			outputWriter: input.outputWriter,
		}
	}
	verbosePrintf(os.Stdout, "Worker %d done\n", workerID)
	wg.Done()
}

// processResultsUtteranceJSON returns the same results if it's single file or listfile, stdout or to a file.
// Each entry is a line following the pattern `utterance_ID \t json_serialization_of_results`.
func processResultsUtteranceJSON(fileResultsChannel <-chan outputs) error {
	for fileResults := range fileResultsChannel {
		uttID := fileResults.uttID
		response := fileResults.responses
		for nSegment, result := range response {
			if str, err := json.Marshal(result); err != nil {
				fmt.Fprintf(os.Stderr, "[Error serializing]: %s_Segment%d\t%v\n", uttID, nSegment, err)
			} else {
				fmt.Fprintf(fileResults.outputWriter, "%s_%d\t%s\n", uttID, nSegment, string(str))
			}
		}
	}

	return nil
}

func processResultsJSON(fileResultsChannel <-chan outputs, pretty bool) {
	for fileResults := range fileResultsChannel {
		var bytes []byte
		var err error

		// Serialize the results
		if pretty {
			bytes, err = json.MarshalIndent(fileResults, "", "  ")
		} else {
			bytes, err = json.Marshal(fileResults)
		}

		// Print results to outputWriter
		if err == nil {
			fmt.Fprint(fileResults.outputWriter, string(bytes))
		} else {
			fmt.Fprintf(os.Stderr, "Error serializing results: %v\n", err)
		}
	}
}

func processResultsTimeline(fileResultsChannel <-chan outputs) {
	// Create formatter
	cfg := timeline.Config{MaxAlternatives: maxAlternatives}
	formatter, err := cfg.CreateFormatter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Internal error creating timeline formatter: %v", err)
		return
	}

	// Continues reading until the fileResultsChannel is closed
	for fileResults := range fileResultsChannel {
		// Flatten the results, the formatter will separate out partials and empty results.
		var all []*cubicpb.RecognitionResult
		for _, resp := range fileResults.responses {
			all = append(all, resp...)
		}

		// Format the results
		output, err := formatter.Format(all)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while formatting results: %v", err)
			continue
		}

		// Format and print the results to the outputWriter
		fmt.Fprintf(fileResults.outputWriter, "Timeline for '%s':\n%s\n\n", fileResults.uttID, output)
	}
}
