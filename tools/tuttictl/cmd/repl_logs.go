package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/hieunyit/tutti/tools/tuttictl/internal/api"
)

var (
	rlFrom       string
	rlTo         string
	rlUser       string
	rlFromBucket string
	rlToBucket   string
	rlFollow     bool
	rlTail       int
)

const replLogsInterval = 2 * time.Second

// parseReplicationRef parses the colon form of a replication id, `user:from:to` or `user:from:to:fromBucket:toBucket`
// (empty user for storage-level replications, e.g. `:main:backup:photos:photos`).
func parseReplicationRef(s string) (*pb.ReplicationID, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 && len(parts) != 5 {
		return nil, fmt.Errorf("invalid replication id %q: expected user:from:to or user:from:to:fromBucket:toBucket", s)
	}
	var fromBucket, toBucket string
	if len(parts) == 5 {
		fromBucket, toBucket = parts[3], parts[4]
	}
	if parts[1] == "" || parts[2] == "" || len(parts) == 5 && (fromBucket == "" || toBucket == "") {
		return nil, fmt.Errorf("invalid replication id %q: storages and buckets must not be empty", s)
	}
	return buildReplicationID(parts[0], parts[1], parts[2], fromBucket, toBucket), nil
}

var replLogsCmd = &cobra.Command{
	Use:   "logs [id]",
	Short: "show the activity log of a replication (running and recent object copies)",
	Long: `Print the most recent copy events of a replication (started, done, retry, failed, skipped). With -f the
command keeps polling every 2 seconds and prints new events until interrupted.

The replication is given either as an id (user:from:to or user:from:to:fromBucket:toBucket; empty user for
storage-level replications) or with the same flags as the other repl commands.

  tuttictl repl logs :main:backup:photos:photos -f --tail 50
  tuttictl repl logs --from main --to backup --from-bucket photos`,
	Args: cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if rlTail < 1 || rlTail > 500 {
			return fmt.Errorf("--tail must be between 1 and 500")
		}
		if len(args) == 1 {
			return nil
		}
		if rlFrom == "" || rlTo == "" {
			return fmt.Errorf("give a replication id or --from and --to")
		}
		if rlToBucket != "" && rlFromBucket == "" {
			return fmt.Errorf("--to-bucket must be set when --from-bucket is set")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		var id *pb.ReplicationID
		if len(args) == 1 {
			var err error
			if id, err = parseReplicationRef(args[0]); err != nil {
				logrus.Fatal(err)
			}
		} else {
			id = buildReplicationID(rlUser, rlFrom, rlTo, rlFromBucket, rlToBucket)
		}
		conn, client := newPolicyClient(ctx)
		defer conn.Close()

		fetch := func(ctx context.Context, after string, limit int32) (*pb.GetReplicationActivityResponse, error) {
			return client.GetReplicationActivity(ctx, &pb.GetReplicationActivityRequest{Id: id, AfterCursor: after, Limit: limit})
		}
		if err := api.PrintActivity(ctx, fetch, os.Stdout, os.Stderr, rlTail, rlFollow, replLogsInterval); err != nil {
			api.PrintGrpcError(err)
		}
	},
}

func init() {
	replCmd.AddCommand(replLogsCmd)
	replLogsCmd.Flags().BoolVarP(&rlFollow, "follow", "f", false, "keep polling and print new events (every 2s)")
	replLogsCmd.Flags().IntVar(&rlTail, "tail", 100, "number of most recent events to show first (1-500)")
	replLogsCmd.Flags().StringVar(&rlFrom, "from", "", "source storage name (when no id is given)")
	replLogsCmd.Flags().StringVar(&rlTo, "to", "", "destination storage name (when no id is given)")
	replLogsCmd.Flags().StringVarP(&rlUser, "user", "u", "", "replication user; omit for storage-level replication")
	replLogsCmd.Flags().StringVarP(&rlFromBucket, "from-bucket", "b", "", "source bucket name; omit for user-level policies")
	replLogsCmd.Flags().StringVar(&rlToBucket, "to-bucket", "", "destination bucket name; defaults to from-bucket")
}
