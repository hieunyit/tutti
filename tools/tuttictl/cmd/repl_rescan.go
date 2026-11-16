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
	"google.golang.org/protobuf/types/known/durationpb"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/hieunyit/tutti/tools/tuttictl/internal/api"
)

var (
	rsFrom       string
	rsTo         string
	rsUser       string
	rsFromBucket string
	rsToBucket   string
	rsEvery      string
)

func addReplIDFlags(c *cobra.Command, from, to, user, fromBucket, toBucket *string) {
	c.Flags().StringVarP(from, "from", "f", "", "source storage name")
	c.Flags().StringVarP(to, "to", "t", "", "destination storage name")
	c.Flags().StringVarP(user, "user", "u", "", "replication user; omit for storage-level replication")
	c.Flags().StringVarP(fromBucket, "from-bucket", "b", "", "source bucket name; omit for user-level policies")
	c.Flags().StringVar(toBucket, "to-bucket", "", "destination bucket name; defaults to from-bucket for bucket-level policies")
	if err := c.MarkFlagRequired("from"); err != nil {
		logrus.WithError(err).Fatal()
	}
	if err := c.MarkFlagRequired("to"); err != nil {
		logrus.WithError(err).Fatal()
	}
}

var rescanCmd = &cobra.Command{
	Use:   "rescan",
	Short: "sync now: rescan the source of a replication immediately",
	Long: `Sync now: list the source bucket right away and copy objects that are missing at the destination.
Fails when a scan is already running, the replication is paused or its initial copy is not finished.

  tuttictl repl rescan --from main --to follower --user admin --from-bucket bucket1`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if rsToBucket != "" && rsFromBucket == "" {
			return fmt.Errorf("--to-bucket must be set when --from-bucket is set")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		conn, client := newPolicyClient(ctx)
		defer conn.Close()

		id := buildReplicationID(rsUser, rsFrom, rsTo, rsFromBucket, rsToBucket)
		if _, err := client.RescanReplication(ctx, id); err != nil {
			api.PrintGrpcError(err)
		}
	},
}

var setRescanCmd = &cobra.Command{
	Use:   "set-rescan",
	Short: "set or turn off the periodic rescan of a replication",
	Long: `Set how often the source is rescanned for files written directly to it (15m to 24h), or turn it off.

  tuttictl repl set-rescan --from main --to follower --user admin --from-bucket bucket1 --every 1h
  tuttictl repl set-rescan --from main --to follower --user admin --from-bucket bucket1 --every off`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if rsToBucket != "" && rsFromBucket == "" {
			return fmt.Errorf("--to-bucket must be set when --from-bucket is set")
		}
		_, err := parseRescanEvery("every", rsEvery, true)
		return err
	},
	Run: func(cmd *cobra.Command, args []string) {
		every, err := parseRescanEvery("every", rsEvery, true)
		if err != nil {
			logrus.WithError(err).Fatal()
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		conn, client := newPolicyClient(ctx)
		defer conn.Close()

		id := buildReplicationID(rsUser, rsFrom, rsTo, rsFromBucket, rsToBucket)
		_, err = client.UpdateReplicationRescanInterval(ctx, &pb.UpdateReplicationRescanIntervalRequest{
			Id:             id,
			RescanInterval: durationpb.New(every),
		})
		if err != nil {
			api.PrintGrpcError(err)
		}
	},
}

func init() {
	replCmd.AddCommand(rescanCmd)
	replCmd.AddCommand(setRescanCmd)
	addReplIDFlags(rescanCmd, &rsFrom, &rsTo, &rsUser, &rsFromBucket, &rsToBucket)
	// separate flag set for set-rescan: bind to the same variables
	addReplIDFlags(setRescanCmd, &rsFrom, &rsTo, &rsUser, &rsFromBucket, &rsToBucket)
	setRescanCmd.Flags().StringVar(&rsEvery, "every", "", "rescan interval: 15m, 30m, 1h, 3h, 6h, 12h, 24h or off")
	if err := setRescanCmd.MarkFlagRequired("every"); err != nil {
		logrus.WithError(err).Fatal()
	}
}
