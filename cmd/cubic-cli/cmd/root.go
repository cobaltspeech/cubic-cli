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
	"fmt"
	"io"
	"os"
	"strings"

	cubic "github.com/cobaltspeech/sdk-cubic/grpc/go-cubic"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cubic-cli",
	Short: "cubic-cli is a command line interface for interacting with a running instance of cubicsvr",
	Long:  `cubic-cli is a command line interface for interacting with a running instance of cubicsvr`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var cubicSvrAddress string
var verbose bool
var insecure bool

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(transcribeCmd)
	rootCmd.AddCommand(rtfCmd)

	rootCmd.PersistentFlags().StringVarP(&cubicSvrAddress, "server", "s", "localhost:2727",
		"Address of running cubicsvr instance.  Format should be 'address:port'.")

	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false,
		"By default, connections to the server are encrypted (TLS).  Include '--insecure' if you want TLS disabled.")

	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false,
		"If true, extra logging will be done.  Helful for debugging.")
	_ = rootCmd.PersistentFlags().MarkHidden("verbose") // Shouldn't fail, since we just added it.
}

// createClient is a helper function that is shared by models.go, transcribe.go, and version.go
func createClient(cubicSvrAddress string, insecure bool) (*cubic.Client, error) {
	var client *cubic.Client
	var err error

	if insecure {
		client, err = cubic.NewClient(cubicSvrAddress, cubic.WithInsecure())
	} else {
		client, err = cubic.NewClient(cubicSvrAddress)
	}

	if err != nil {
		return nil, fmt.Errorf("unable to create client for --server '%s'", cubicSvrAddress)
	}

	return client, nil
}

// verbosePrintf is a helper function.  Use --verbose to allow these strings to be printed
func verbosePrintf(w io.Writer, format string, a ...interface{}) {
	if verbose {
		fmt.Fprintf(w, "DEBUG: "+format, a...)
	}
}

// simplifyGrpcErrors converts semi-cryptic gRPC errors into more user-friendly errors.
func simplifyGrpcErrors(msg string, err error) error {
	// TODO create more robust/consistent ways of checking for each error.  This is a little too ad-hoc.
	switch {
	case strings.Contains(err.Error(), "transport: Error while dialing dial tcp"):
		return fmt.Errorf(msg+": unable to reach server at the address '%s'", cubicSvrAddress)

	case strings.Contains(err.Error(), "authentication handshake failed: tls:"):
		return fmt.Errorf(msg + ": '--insecure' required for this connection")

	case strings.Contains(err.Error(), "desc = all SubConns are in TransientFailure, latest connection error: "):
		return fmt.Errorf(msg + ": '--insecure' must not be used for this connection")

	case strings.Contains(err.Error(), "invalid model requested"):
		return fmt.Errorf(msg+": invalid --model '%s' (%v)", model, err)

	default:
		return fmt.Errorf(msg + ": " + err.Error()) // return the grpc error directly
	}

}
