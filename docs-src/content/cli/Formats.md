---
title: "Output formats"
weight: 30
---

There are four different output options supported:

* [timeline](#timeline)
* [json](#json-and-json-pretty)
* [json-pretty](#json-and-json-pretty)
* [utterance-json](#utterance-json)

All the output options return a list of JSON representations of each result recognized in the
transcript. Cubic can be configured with different endpoint settings to determine how long a 
pause between words will begin a new result; the default is half a second.

<!--more-->

## Timeline

Timeline is the default output format. For multi-channel audio, it sorts the results from different channels by their start times, showing the conversation in order.  

### Fields
* **channel_id**: Which channel produced the results
* **nbest**: The alternatives recognized for each result.  By default, only the most likely alternative is returned for each result.  Specify `--fmt.timeline.maxAlts` to include additional, lower confidence, results.
    * **start_time**: Number of milliseconds since the beginning of the audio file
    * **duration**: Number of milliseconds for the entire result
    * **confidence**: Number between 0 and 1 representing the probability of this result vs the other alternatives
    * **transcript**: Full result (up to the endpoint)
    * **words**: Array of the individual confidence, duration, and start time for each word in the transcript.  Can be used to provide finer alignment to the audio.

### Example
```bash
bin/cubic-cli -s localhost:2727 transcribe -m 1 -f timeline --stereo --insecure 2-channel.wav
```
<details>
<summary><font color="236ecc"><b>Click here to see sample output</b></font></summary>

```json
[
  {
    "channel_id": 0,
    "nbest": [
      {
        "words": [
          {
            "start_time": 1980,
            "duration": 240,
            "word": "Thank",
            "confidence": 1
          },
          {
            "start_time": 2220,
            "duration": 120,
            "word": "you",
            "confidence": 1
          },
          {
            "start_time": 2340,
            "duration": 120,
            "word": "for",
            "confidence": 1
          },
          {
            "start_time": 2460,
            "duration": 330,
            "word": "calling",
            "confidence": 1
          },
          {
            "start_time": 2800,
            "duration": 470,
            "word": "Acme.",
            "confidence": 0.943
          },
          {
            "start_time": 3660,
            "duration": 120,
            "word": "How",
            "confidence": 1
          },
          {
            "start_time": 3780,
            "duration": 180,
            "word": "may",
            "confidence": 1
          },
          {
            "start_time": 3960,
            "duration": 60,
            "word": "I",
            "confidence": 1
          },
          {
            "start_time": 4020,
            "duration": 270,
            "word": "direct",
            "confidence": 1
          },
          {
            "start_time": 4290,
            "duration": 120,
            "word": "your",
            "confidence": 1
          },
          {
            "start_time": 4410,
            "duration": 450,
            "word": "call.",
            "confidence": 1
          }
        ],
        "start_time": 1980,
        "duration": 2880,
        "confidence": 0.902,
        "transcript": "Thank you for calling Acme. How may I direct your call."
      }
    ]
  },
  {
    "channel_id": 1,
    "nbest": [
      {
        "words": [
          {
            "start_time": 6180,
            "duration": 300,
            "word": "Mary",
            "confidence": 0.877
          },
          {
            "start_time": 7128,
            "duration": 462,
            "word": "please.",
            "confidence": 1
          }
        ],
        "start_time": 6180,
        "duration": 1410,
        "confidence": 0.362,
        "transcript": "Mary please."
      }
    ]
  },
  {
    "channel_id": 0,
    "nbest": [
      {
        "words": [
          {
            "start_time": 8880,
            "duration": 600,
            "word": "Certainly",
            "confidence": 1
          },
          {
            "start_time": 9780,
            "duration": 240,
            "word": "please",
            "confidence": 1
          },
          {
            "start_time": 10020,
            "duration": 210,
            "word": "hold",
            "confidence": 1
          },
          {
            "start_time": 10230,
            "duration": 180,
            "word": "on",
            "confidence": 1
          },
          {
            "start_time": 10410,
            "duration": 120,
            "word": "for",
            "confidence": 1
          },
          {
            "start_time": 10530,
            "duration": 112,
            "word": "just",
            "confidence": 0.622
          },
          {
            "start_time": 10664,
            "duration": 114,
            "word": "a",
            "confidence": 0.744
          },
          {
            "start_time": 10778,
            "duration": 532,
            "word": "minute.",
            "confidence": 1
          }
        ],
        "start_time": 8880,
        "duration": 2430,
        "confidence": 0.461,
        "transcript": "Certainly please hold on for just a minute."
      }
    ]
  }
]
```
</details>

## Json and Json-pretty

Both `json` and `json-pretty` return the same data.
The only difference is that `json-pretty` includes line breaks and indentation to make it more human-readable.
Both formats return all alternatives recognized for each result, with the default JSON encoding for
the [RecognitionAlternative](/sdk-cubic/protobuf/autogen-doc-cubic-proto/#message-recognitionalternative) 
object. That encoding makes it straightforward to deserialize start_time and duration fields into 
[google.protobuf.Duration](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Duration) objects, but may not be as human-readable because the JSON encoding for
Duration objects has separate fields for seconds and nanoseconds.

Word-level information is not included and the results are the order cubic processed them, so results from 
different channels are not guaranteed to be ordered according to their timestamps.

### Example
```bash
bin/cubic-cli -s demo-cubic.cobaltspeech.com:2727 transcribe -m 1 -f json-pretty testdata/this_is_a_test-en_us-16.wav 
```

<details>
<summary><font color="236ecc"><b>Click here to see sample output</b></font></summary>

```json
{
  "UttID": "this_is_a_test-en_us-16.wav",
  "Responses": [
    {
      "alternatives": [
        {
          "transcript": "This is a test.",
          "confidence": 0.994,
          "start_time": {
            "nanos": 900000000
          },
          "duration": {
            "seconds": 1,
            "nanos": 50000000
          }
        },
        {
          "transcript": "Hi this is a test.",
          "confidence": 0.006,
          "start_time": {
            "nanos": 5000000
          },
          "duration": {
            "seconds": 1,
            "nanos": 945000000
          }
        }
      ]
    }
  ]
}
```
</details>

## Utterance-json

Utterance-json is a tab-delimited format, included for backward compatibility with asr_tool, the previous command-line test utility for cubic. 

Each result is a separate line with the identifier separated from the json representation of the result 
by a tab character:
```
<utteranceId>_<resultIndex> <tab> {<result json>}
```

### Example
```bash
bin/cubic-cli -s demo-cubic.cobaltspeech.com:2727 transcribe -m 1 -f utterance-json --list-file testdata/list.txt 
```

<details>
<summary><font color="236ecc"><b>Click here to see sample output</b></font></summary>

```
Utterance_1_0	{"alternatives":[{"transcript":"This is a test.","confidence":0.994,"start_time":{"nanos":900000000},"duration":{"seconds":1,"nanos":50000000}},{"transcript":"Hi this is a test.","confidence":0.006,"start_time":{"nanos":5000000},"duration":{"seconds":1,"nanos":945000000}}]}
Utterance_1_1	{"alternatives":[{"transcript":"","confidence":0.731},{"transcript":"Yeah.","confidence":0.163,"start_time":{"seconds":3,"nanos":261000000},"duration":{"seconds":1,"nanos":801000000}},{"transcript":"I.","confidence":0.039,"start_time":{"seconds":3,"nanos":261000000},"duration":{"seconds":1,"nanos":801000000}},{"transcript":"Hey.","confidence":0.036,"start_time":{"seconds":3,"nanos":261000000},"duration":{"seconds":1,"nanos":801000000}},{"transcript":"Bye.","confidence":0.032,"start_time":{"seconds":3,"nanos":261000000},"duration":{"seconds":1,"nanos":801000000}}]}
Utterance_2_0	{"alternatives":[{"transcript":"The second test.","confidence":0.6,"start_time":{"nanos":944000000},"duration":{"seconds":1,"nanos":96000000}},{"transcript":"What the second test.","confidence":0.154,"start_time":{},"duration":{"seconds":2,"nanos":40000000}},{"transcript":"At the second test.","confidence":0.134,"start_time":{},"duration":{"seconds":2,"nanos":40000000}},{"transcript":"But the second test.","confidence":0.09,"start_time":{},"duration":{"seconds":2,"nanos":40000000}},{"transcript":"Is the second test.","confidence":0.022,"start_time":{},"duration":{"seconds":2,"nanos":40000000}}]}
```
</details>
