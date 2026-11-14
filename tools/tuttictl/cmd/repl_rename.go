/*
 * Copyright © 2023 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/hieunyit/tutti/tools/tuttictl/internal/api"
)

var (
	rnFrom       string
	rnTo         string
	rnUser       string
	rnFromBucket string
	rnToBucket   string
	rnName       string
)

// renameCmd represents the pause command
var renameCmd = &cobra.Command{
	Use:   "rename",
	Short: "rename a replication (display name only)",
	Long: `Set or clear the display name of a replication. The name is a label only: it does not affect copying or routing. An empty --name clears it.

User-level policy:
  tuttictl repl rename --from main --to follower --user admin

Bucket-level policy:
  tuttictl repl rename --from main --to follower --user admin --from-bucket bucket1 --name "prod to dr"`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if rnToBucket != "" && rnFromBucket == "" {
			return fmt.Errorf("--to-bucket must be set when --from-bucket is set")
		}
		if err := checkReplName(rnName); err != nil {
			return err
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		conn, client := newPolicyClient(ctx)
		defer conn.Close()

		id := buildReplicationID(rnUser, rnFrom, rnTo, rnFromBucket, rnToBucket)
		name := strings.TrimSpace(rnName)
		_, err := client.UpdateReplicationName(ctx, &pb.UpdateReplicationNameRequest{Id: id, Name: name})
		if err != nil {
			api.PrintGrpcError(err)
		}
	},
}

func init() {
	replCmd.AddCommand(renameCmd)
	renameCmd.Flags().StringVar(&rnName, "name", "", "new display name (max 64 characters); empty clears it")
	renameCmd.Flags().StringVarP(&rnFrom, "from", "f", "", "source storage name")
	renameCmd.Flags().StringVarP(&rnTo, "to", "t", "", "destination storage name")
	renameCmd.Flags().StringVarP(&rnUser, "user", "u", "", "replication user; omit for storage-level replication")
	renameCmd.Flags().StringVarP(&rnFromBucket, "from-bucket", "b", "", "source bucket name; omit for user-level policies")
	renameCmd.Flags().StringVar(&rnToBucket, "to-bucket", "", "destination bucket name; defaults to from-bucket for bucket-level policies")

	if err := renameCmd.MarkFlagRequired("from"); err != nil {
		logrus.WithError(err).Fatal()
	}
	if err := renameCmd.MarkFlagRequired("to"); err != nil {
		logrus.WithError(err).Fatal()
	}
}
