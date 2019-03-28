# cubic-cli

## Overview

This folder (and resulting binary) can be used to send audio file(s) to a running instance of a cubic server.  It consists of several sub-commands:

| `cubic-cli`            | Prints the default help message. |
| `cubic-cli models`     | Displays the transcription models being served by the given instance. |
| `cubic-cli version`    | Displays the versions of both the client and the server. |
| `cubic-cli transcribe` | Sends audio file(s) to server for transcription. |

`[cmd] --help` can be run on any command for more details on usage and included flags.

## Building

The `Makefile` should provide all necessary commands.
Specifically `make build`.

TODO: Make this compatable with the `go get` or `go install` process.

## Running/testing

There are two audio files in the testdata folder.
They are US English, recorded at 16kHz.
The filenames contain the intended transcription.

As a quick start, these commands are provided as examples of how to use the binary.  They should be run from the current directory (`cli-client/`).

Note: These commands assume that the cubic server instance is available at `localhost:2727`.  For initial testing, there is a demo server available at `http://demo-cubic.cobaltspeech.com:2727`.

```sh
# Display the versions of client and server
cubic-cli version \
    --server localhost:2727

# List available models.  Note: The listed modelIDs are used in transcription methods
cubic-cli models \
    --server localhost:2727

# Transcribe the single file this_is_a_test-en_us-16.wav.
## Should result in the transcription of "this is a test"
bin/cubic-cli transcribe \
    ./testdata/this_is_a_test-en_us-16.wav \
    --server localhost:2727 \

# Transcribe the list of files defined at ./testdata/list.txt
## Should result in the transcription of "this is a test" and "the second test" printed to stdout
bin/cubic-cli transcribe --list-file \
    ./testdata/list.txt \
    --server localhost:2727 \

# Same as the previous `transcribe` command, but redirects the results to the --outputFile.
bin/cubic-cli transcribe --list-file \
    ./testdata/list.txt \
    --server localhost:2727 \
    --outputFile ./testdata/out.txt \

# Same as the first `transcribe` command, but sends up to two files at a time.
## Note that the server may place a limit to the maximum number of concurrent requests processed.
bin/cubic-cli transcribe --list-file \
    ./testdata/list.txt \
    --server localhost:2727 \
    --workers 2
```
