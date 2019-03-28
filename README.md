# cubic-cli

## Overview

This folder (and resulting binary) can be used to send audio file(s) to a running instance of a cubic server.  It consists of several sub-commands:

| Command                | Purpose |
| ---------------------- | ------- |
| `cubic-cli`            | Prints the default help message. |
| `cubic-cli models`     | Displays the transcription models being served by the given instance. |
| `cubic-cli version`    | Displays the versions of both the client and the server. |
| `cubic-cli transcribe` | Sends audio file(s) to server for transcription. |

`[cmd] --help` can be run on any command for more details on usage and included flags.

## Building

The `Makefile` should provide all necessary commands.
Specifically `make build`.

TODO: Make this compatable with the `go get` or `go install` process.

## Running

There are two audio files in the testdata folder.
They are US English, recorded at 16kHz.
The filenames contain the intended transcription.

As a quick start, these commands are provided as examples of how to use the
binary.  They should be run from the current directory (`cubic-cli`).

Note: These commands assume that the your instance of cubic server is available
at `localhost:2727`.

```sh
# Display the versions of client and server
./bin/cubic-cli --insecure --server localhost:2727 version

# List available models.  Note: The listed modelIDs are used in transcription methods
./bin/cubic-cli --insecure --server localhost:2727 models

# Transcribe the single file this_is_a_test-en_us-16.wav.
## Should result in the transcription of "this is a test"
./bin/cubic-cli --insecure --server localhost:2727 \
	transcribe ./testdata/this_is_a_test-en_us-16.wav

# Transcribe the list of files defined at ./testdata/list.txt
## Should result in the transcription of "this is a test" and "the second test" printed to stdout
./bin/cubic-cli --insecure --server localhost:2727 \
	transcribe --list-file ./testdata/list.txt

# Same as the previous `transcribe` command, but redirects the results to the --outputFile.
./bin/cubic-cli --insecure --server localhost:2727 \
	transcribe --list-file ./testdata/list.txt \
	--outputFile ./testdata/out.txt

# Same as the first `transcribe` command, but sends up to two files at a time.
## Note that the server may place a limit to the maximum number of concurrent requests processed.
./bin/cubic-cli --insecure --server localhost:2727 \
	transcribe --list-file ./testdata/list.txt \
    --workers 2
```

For quick testing, Cobalt's demo server can be accessed with `--server
demo-cubic.cobaltspeech.com:2727`. This uses TLS and does not need the
`--insecure` flag.

Commercial use of the demo service is not permitted. This server is for testing
and demonstration purposes only and is not guaranteed to support high
availability or high volume. Data uploaded to the server may be stored for
internal purposes.
