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

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/hieunyit/tutti/tools/tuttictl/internal/api"
)

var (
	raFrom        string
	raTo          string
	raUser        string
	raEventSource string
	raFromBucket  string
	raToBucket    string
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "create replication policy",
	Long: `Create replication policy. --user is optional: without it the replication is storage-level (whole storage A to B, using each storage connection account).


User-level replication (all existing and future buckets for a user):
  tuttictl repl add --from main --to follower --user admin

Bucket-level replication (single bucket, same name on both storages):
  tuttictl repl add --from main --to follower --user admin --from-bucket bucket1

Bucket-level replication (different destination bucket name):
  tuttictl repl add --from main --to follower --user admin --from-bucket src-bucket --to-bucket dest-bucket

S3 notification event source (worker creates SNS topic + bucket notification):
  tuttictl repl add --from main --to follower --user admin --from-bucket bucket1 --event-source s3-notification

External webhook event source (e.g. Swift access log exporter):
  tuttictl repl add --from main --to follower --user admin --event-source webhook`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if raToBucket != "" && raFromBucket == "" {
			return fmt.Errorf("--to-bucket must be set when --from-bucket is set")
		}
		if raEventSource != "" && raEventSource != "proxy" && raEventSource != "s3-notification" && raEventSource != "webhook" {
			return fmt.Errorf("--event-source must be one of: proxy, s3-notification, webhook")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		conn, client := newPolicyClient(ctx)
		defer conn.Close()

		id := buildReplicationID(raUser, raFrom, raTo, raFromBucket, raToBucket)
		req := &pb.AddReplicationRequest{Id: id}
		if raEventSource != "" && raEventSource != "proxy" {
			req.Opts = &pb.ReplicationOpts{}
			switch raEventSource {
			case "s3-notification":
				req.Opts.EventSource = pb.EventSource_EVENT_SOURCE_S3_NOTIFICATION
			case "webhook":
				req.Opts.EventSource = pb.EventSource_EVENT_SOURCE_WEBHOOK
			}
		}

		_, err := client.AddReplication(ctx, req)
		if err != nil {
			api.PrintGrpcError(err)
		}
	},
}

func init() {
	replCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&raFrom, "from", "f", "", "source storage name")
	addCmd.Flags().StringVarP(&raTo, "to", "t", "", "destination storage name")
	addCmd.Flags().StringVarP(&raUser, "user", "u", "", "replication user; omit for storage-level replication")
	addCmd.Flags().StringVar(&raEventSource, "event-source", "", "event source: proxy, s3-notification, webhook")
	addCmd.Flags().StringVarP(&raFromBucket, "from-bucket", "b", "", "source bucket name; omit for user-level policies")
	addCmd.Flags().StringVar(&raToBucket, "to-bucket", "", "destination bucket name; defaults to from-bucket for bucket-level policies")

	if err := addCmd.MarkFlagRequired("from"); err != nil {
		logrus.WithError(err).Fatal()
	}
	if err := addCmd.MarkFlagRequired("to"); err != nil {
		logrus.WithError(err).Fatal()
	}
}
