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
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	commitHash string
	version    string
)

var versionCmd = &cobra.Command{
	Use:           "version [--server address:port] [--insecure]",
	Short:         "Report cubicsvr version and return.",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Print the client version
		fmt.Fprint(os.Stdout, clientVersion())

		// Print the server error
		v, err := serverVersion()
		if v != "" {
			fmt.Fprint(os.Stdout, v)
		}

		return err
	},
}

func clientVersion() string {
	if version == "" {
		version = "devel"
	}
	if commitHash == "" {
		commitHash = "nil"
	}
	return fmt.Sprintf("Client: (Version: %s -- Commit: %s)\n", version, commitHash)
}

func serverVersion() (string, error) {
	// Create client connection
	verbosePrintf(os.Stdout, "Creating connection to server '%s'\n", cubicSvrAddress)
	client, err := createClient()
	if err != nil {
		return "", err
	}
	defer client.Close()

	// Request the server version
	verbosePrintf(os.Stdout, "Fetching version from server\n")
	resp, err := client.Version(context.Background())
	if err != nil {
		verbosePrintf(os.Stderr, "Error fetching cubicsvr version: %v\n", err)
		return "", simplifyGrpcErrors("Error fetching cubicsvr version", err)
	}
	return fmt.Sprintf("Cubic Server: (Cubic: %s -- Server: %s)\n", resp.Cubic, resp.Server), nil
}
