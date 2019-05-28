---
title: "Cubic Command Line Tool Documentation"
---

# Cubic Overview

Cubic is Cobalt’s automatic speech recognition (ASR) engine. It can be deployed on-prem and accessed over the network or on your local machine via this command-line tool. The tool is written using our public-facing [SDK](https://cobaltspeech.github.io/sdk-cubic/) so the source code provides an example of how to use the SDK.

## Formatted output

Speech recognition systems typically output the words that were spoken, with no formatting. For example, utterances with numbers in might return “twenty seven bridges”, and “the year two thousand and three”. Cubic has the option to enable basic formatting of speech recognition results:

* Capitalising the first letter of the utterance
* Numbers: “cobalt’s atomic number is twenty seven” -> “Cobalt’s atomic number is 27”
* Truecasing: “the iphone was launched in two thousand and seven” -> “The iPhone was launched in 2007”
* Ordinals: “summer solstice is twenty first june” -> “Summer solstice is 21st June”
* Punctuation: "from mid april on the rain has ben incessant" -> "From mid-April on, the rain has been incessant."

The cubic server configuration determines which of these formatting rules will apply; for instance, you might choose to enable punctuation but not change number words or vice versa depending on how the output will be used.

## Obtaining Cubic

Cobalt will provide you with a package of Cubic that contains the engine,
appropriate speech recognition models and a server application.  This server
exports Cubic's functionality over the gRPC protocol.  The
https://github.com/cobaltspeech/sdk-cubic repository contains the SDK that you
can use in your application to communicate with the Cubic server. This SDK is
currently available for the Go and Python languages; and we would be happy to talk to you if
you need support for other languages. Most of the core SDK is generated
automatically using the gRPC tools, and Cobalt provides a top level package for
more convenient API calls.
