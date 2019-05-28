---
title: "CLI Overview"
weight: 10
---

The cubic-cli binary provides a convenient way to generate transcripts from an audio file in order to evaluate a model.  It accepts either a single audio file or a list of files, sends the audio to a running instance of cubicsvr, and outputs the transcript in one of several formats.

<!--more-->

| Command                | Purpose |
| ---------------------- | ------- |
| `cubic-cli`            | Prints the default help message. |
| `cubic-cli models`     | Displays the transcription models being served by the given instance. |
| `cubic-cli version`    | Displays the versions of both the client and the server. |
| `cubic-cli transcribe` | Sends audio file(s) to server for transcription. |

`[cmd] --help` can be run on any command for more details on usage and included flags.

## Global Flags

All commands accept the following flags

| Short | Long form      | Arg | Purpose |
| ------|--------------- | ---- | ------- |
| |      `--insecure`      | none | By default, connections to the server are encrypted (TLS).  Include`'--insecure` if you want TLS disabled. |
| -s | `--server` | string | Address of running cubicsvr instance.  Format should be 'address:port'. (default "localhost:2727") |
| -h | `--help` | none | Display more details on usage and available flags |

## Examples

Note: These commands assume that the your instance of cubic server is available
at `localhost:2727` and that the command is being run from the root of the cubic-cli directory.

If you do not have a local instance, Cobalt's demo server can be accessed with `--server
demo-cubic.cobaltspeech.com:2727`. This uses TLS and does not need the
`--insecure` flag.

> Commercial use of the demo service is not permitted. This server is for testing
and demonstration purposes only and is not guaranteed to support high
availability or high volume. Data uploaded to the server may be stored for
internal purposes.

```sh
# Display the versions of client and server
./bin/cubic-cli --insecure --server localhost:2727 version

# List available models.  Note: The listed modelIDs are used in transcription methods
./bin/cubic-cli --insecure --server localhost:2727 models

# Transcribe the single file this_is_a_test-en_us-16.wav using modelId1.
# (On the demo server, modelId 1 is the U.S. English 16 kHz model--the model id depends on the
# cubic server configuration.)
# Should result in the transcription of "this is a test"
./bin/cubic-cli --insecure --server localhost:2727 -m 1 transcribe ./testdata/this_is_a_test-en_us-16.wav
```

For more examples of the transcribe command, see [Transcribe](/sdk-cubic/cli/transcribe)
