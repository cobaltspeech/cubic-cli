---
title: "Transcribe Command"
weight: 20
---

This command is used for transcribing audio files.
There are two modes: single file or list file.

Usage:
```
transcribe FILE_PATH [flags]
```

In single mode, FILE_PATH should point to a single audio file.

In list-file mode, FILE_PATH should point to a file listing multiple audio files with the format "Utterance_ID \t FILE_PATH \n". 
    
Audio files in the following formats are supported:
    WAV, FLAC, MP3, VOX, and RAW(PCM16SLE).

The file extension (wav, flac, mp3, vox, raw) will be used to determine which
 codec to use.  Use WAV or FLAC for best results.

<!--more-->

| Short | Long form      | Arg | Purpose |
| ----- | -------------- | --- | ------- |
| -l | `--list-file` | none | Indicates that FILE_PATH is pointing to a list of audio files. |
| -o | `--output` | string | Path to directory where the results should be written.<br/>"-" indicates stdout. (default "-") |
| -m | `--model` | string | Selects which model ID to use for transcribing. (default "1") |
| -c | `--audioChannels` | ints   | Audio channels to transcribe.  (Defaults to mono) |
| | `--stereo` | none |  Sets `--audioChannels "0,1"` to transcribe both audio channels of a stereo file.|
| -n | `--workers` | int | Number of concurrent requests to send to cubicsvr. (default 1) |
| -f | `--outputFormat` | string | Format of output.  Can be [json,json-pretty,timeline,utterance-json]. (default "timeline") |
| -a | `--fmt.timeline.maxAlts` | int | Maximum number of alternatives to provide for each result in the timeline format. (default 1) |


### List-file

To process multiple files, use the `--list-file` flag and specify a text file instead of an audio file.
The text file should be tab-delimited, with one audio file per line.  Each line should begin with a unique
identifier containing no white space, which will be used to identify which transcript belongs to which 
input file.  

Example:
```
utterance1  /mnt/audio/abcd.wav
utterance2  /mnt/audio/efgh.wav
```

When cubic-cli writes to stdout (`-o "-"`), each output will be prefaced by the utterance id and an extra newline. 

### Output

Output must be an existing directory to which the transcripts will be written.

In single-file mode, the transcript will be given the same name as the audio file, with a .txt extension.

In list-file mode, each file processed will have a separate output file, using the utteranceID as the filename with a ".txt" extention.

In either mode, "-" prints to stdout, which is the default behavior.

### Model

A cubicsvr instance may be configured to use one or more models, each with a model id specified in cubicsvr.cfg.toml. The specified id must be one of the ones configured for the cubicsvr instance.

For example, if connecting to the demo server, model=1 specifies the U.S. English 16 kHz model and model=2 specifies the U.S. English 8 kHz model.

The models for a server may be listed by calling `./bin/cubic-cli --insecure --server localhost:2727 models`.

Example:
```bash
$ bin/cubic-cli -s demo-cubic.cobaltspeech.com:2727 models
Model ID '"1"': English (US) 16KHz (Sample Rate: 16000)
Model ID '"2"': English (US) 8KHz (Sample Rate:  8000)
Model ID '"3"': Portuguese (BR) 8KHz (Sample Rate:  8000)
Model ID '"4"': Spanish (ES) 16KHz (Sample Rate: 16000)

$ bin/cubic-cli -s demo-cubic.cobaltspeech.com:2727 transcribe -m 2 sample-8khz.wav 
```

### Audio channels

By default, cubic-svr assumes the input is a mono file and transcribes channel 0.
Pass `--audioChannels` to specify other channels.

E.g. `-c "0,2"` would transcribe the first and third channels in the file.

`--stereo` is equivalent to `-c 0,1`

`-c` or `--audioChannel` overrides `--stereo` if both are provided.

### Workers

The `workers` parameter limits the number of requests to send to cubicsvr at once.  

Please note, while this value is defined client-side, the throughput will be limited by 
the available computational ability of the server.  If you are the only connection to an 
8-core server, then "-n 8" is a reasonable value. 

### Output format

See [Formats](/sdk-cubic/cli/formats)

## Examples

There are two audio files in the testdata folder.
They are US English, recorded at 16kHz.
The filenames contain the intended transcription.

As a quick start, these commands are provided as examples of how to use the
binary.  They should be run from the root directory of `cubic-cli`.

Note: These examples show commands both for a local instance 
(assuming cubic server is running at localhost:2727) and for calling Cobalt's demo server.
(`--server demo-cubic.cobaltspeech.com:2727`). The demo server uses TLS and does not need the
`--insecure` flag. Any of the examples could be run against either server. 
All examples use the default modelId (1), which is the U.S. English 16 kHz model on the 
demo server, but there may be a different model in the local instance which is not a good fit 
for the sample audio files provided.

> Commercial use of the demo service is not permitted. This server is for testing
and demonstration purposes only and is not guaranteed to support high
availability or high volume. Data uploaded to the server may be stored for
internal purposes.

```sh
# Display the versions of client and server
./bin/cubic-cli --insecure --server localhost:2727 version

# List available models.  Note: The listed modelIDs are used in transcription methods
./bin/cubic-cli --insecure --server localhost:2727 models

# Transcribe the single file this_is_a_test-en_us-16.wav.
# Should result in the transcription of "this is a test"
./bin/cubic-cli --insecure --server localhost:2727 \
    transcribe ./testdata/this_is_a_test-en_us-16.wav

# Transcribe the list of files defined at ./testdata/list.txt
# Should result in the transcription of "this is a test" and "the second test" printed to stdout
./bin/cubic-cli --insecure --server localhost:2727 \
    transcribe --list-file ./testdata/list.txt

# Same as the previous `transcribe` command, but will create two text files in thie testdata directory
# and may send up to two files at a time to the demo server
./bin/cubic-cli --server demo-cubic.cobaltspeech.com:2727 \
    transcribe --list-file ./testdata/list.txt \
    --output ./testdata --workers 2

# Transcribe multiple channels. Note: this is the same as '--stereo'
./bin/cubic-cli --server demo-cubic.cobaltspeech.com:2727 \
    transcribe ./testdata/this_is_a_test-en_us-16.wav \
    --audioChannels 0,1

# Select different output format.  --outputFormat json allows you to easily 
# pipe the results to another program.
./bin/cubic-cli --insecure --server localhost:2727 \
    transcribe ./testdata/this_is_a_test-en_us-16.wav \
    --outputFormat json
```
