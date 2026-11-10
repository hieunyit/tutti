package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
	"github.com/hieunyit/tutti/tools/tuttictl/internal/api"
)

func resilienceClient(ctx context.Context) (pb.ResilienceClient, func()) {
	conn, err := api.Connect(ctx, address, insecureTLS)
	if err != nil {
		logrus.WithError(err).WithField("address", address).Fatal("unable to connect to api")
	}
	return pb.NewResilienceClient(conn), func() { conn.Close() }
}

var proxiesCmd = &cobra.Command{
	Use:   "proxies",
	Short: "show live proxies with storage health and settings",
	Long: `Shows every live proxy instance, the circuit breaker state of its storages and its effective settings.

Example:
tuttictl proxies`,
	Run: func(_ *cobra.Command, _ []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client, closeFn := resilienceClient(ctx)
		defer closeFn()
		res, err := client.GetProxyStatus(ctx, &emptypb.Empty{})
		if err != nil {
			logrus.WithError(err).Fatal("unable to get proxy status")
		}
		w := tabwriter.NewWriter(os.Stdout, 10, 1, 5, ' ', 0)
		fmt.Fprintln(w, "APP ID\tADDRESS\tUPDATED\tSTORAGES\tSYNC\tFAILOVER")
		for _, p := range res.Proxies {
			var st []string
			for _, s := range p.Storages {
				state := "up"
				if !s.Available {
					state = "DOWN"
				}
				st = append(st, s.Name+"="+state)
			}
			f := p.Settings.GetFailover()
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%t\t%t\n", p.AppId, p.Address,
				p.UpdatedAt.AsTime().Local().Format(time.TimeOnly), strings.Join(st, ","),
				p.Settings.GetReplication().GetSync().GetEnabled(), f.GetEnabled())
		}
		w.Flush()
	},
}

var failoverCmd = &cobra.Command{
	Use:   "failover",
	Short: "inspect automatic failover policies and events",
}

var failoverPoliciesCmd = &cobra.Command{
	Use:   "policies",
	Short: "list failover policies",
	Run: func(_ *cobra.Command, _ []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client, closeFn := resilienceClient(ctx)
		defer closeFn()
		res, err := client.ListFailoverPolicies(ctx, &emptypb.Empty{})
		if err != nil {
			logrus.WithError(err).Fatal("unable to get failover policies")
		}
		w := tabwriter.NewWriter(os.Stdout, 10, 1, 5, ' ', 0)
		fmt.Fprintln(w, "SOURCE\tENABLED\tTARGETS\tAFTER\tMAX EVENT LAG")
		for _, p := range res.Policies {
			lag := "default"
			if p.MaxEventLag != nil {
				lag = fmt.Sprint(*p.MaxEventLag)
			}
			after := "default"
			if d := p.After.AsDuration(); d > 0 {
				after = d.String()
			}
			fmt.Fprintf(w, "%s\t%t\t%s\t%s\t%s\n", p.Source, p.Enabled, strings.Join(p.Targets, ","), after, lag)
		}
		w.Flush()
	},
}

var failoverEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "list performed failovers",
	Run: func(_ *cobra.Command, _ []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client, closeFn := resilienceClient(ctx)
		defer closeFn()
		res, err := client.ListFailoverEvents(ctx, &emptypb.Empty{})
		if err != nil {
			logrus.WithError(err).Fatal("unable to get failover events")
		}
		w := tabwriter.NewWriter(os.Stdout, 10, 1, 5, ' ', 0)
		fmt.Fprintln(w, "USER\tBUCKET\tFROM\tTO\tTIME\tEVENT LAG")
		for _, e := range res.Events {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n", e.User, e.Bucket, e.From, e.To,
				e.Time.AsTime().Local().Format(time.DateTime), e.EventLag)
		}
		w.Flush()
	},
}

var (
	migrateSource, migrateMode string
	migrateApply, migrateForce bool
	migrateElectorate          []string
)

var failoverMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrate a legacy failover policy to protection groups",
	Long: `Shows (dry run) or applies the migration of the legacy failover policy of a source storage to protection groups.
One group is proposed per scope (user or bucket) the legacy controller fails over, adopting the existing replications
(no data is copied). With --apply the groups are created and the legacy policy is disabled once every scope is migrated,
or immediately with --force (uncovered scopes are listed and lose automatic failover). The command is idempotent.`,
	Run: func(_ *cobra.Command, _ []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		conn, err := api.Connect(ctx, address, insecureTLS)
		if err != nil {
			logrus.WithError(err).WithField("address", address).Fatal("unable to connect to api")
		}
		defer conn.Close()
		client := pb.NewProtectionGroupsClient(conn)
		req := &pb.FailoverMigrationRequest{Source: migrateSource, Force: migrateForce, Mode: migrateMode, Electorate: migrateElectorate}
		var plan *pb.FailoverMigrationPlan
		var res *pb.FailoverMigrationResult
		if migrateApply {
			res, err = client.ApplyFailoverMigration(ctx, req)
			if err != nil {
				logrus.WithError(err).Fatal("unable to apply the migration")
			}
			plan = res.Plan
		} else if plan, err = client.PlanFailoverMigration(ctx, req); err != nil {
			logrus.WithError(err).Fatal("unable to plan the migration")
		}
		w := tabwriter.NewWriter(os.Stdout, 10, 1, 5, ' ', 0)
		fmt.Fprintln(w, "USER\tBUCKET\tGROUP\tMEMBERS\tSTATUS\tREASON")
		for _, sc := range plan.Scopes {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", sc.User, sc.Bucket, sc.Group.GetId(), strings.Join(sc.Group.GetMembers(), ","), sc.Status, sc.Reason)
		}
		w.Flush()
		for _, n := range plan.NotCovered {
			fmt.Println("not covered:", n)
		}
		if res == nil {
			fmt.Println("dry run: nothing changed; use --apply to create the groups")
			return
		}
		fmt.Printf("created: %s\nalready migrated: %s\nlegacy policy disabled: %t\n", strings.Join(res.Created, ","), strings.Join(res.Migrated, ","), res.PolicyDisabled)
		if len(res.Uncovered) != 0 {
			fmt.Printf("%d scope(s) not migrated\n", len(res.Uncovered))
		}
	},
}

func init() {
	failoverMigrateCmd.Flags().StringVar(&migrateSource, "source", "", "source storage of the legacy failover policy")
	failoverMigrateCmd.Flags().BoolVar(&migrateApply, "apply", false, "create the groups (default: dry run)")
	failoverMigrateCmd.Flags().BoolVar(&migrateForce, "force", false, "with --apply: disable the legacy policy even if some scopes could not be migrated")
	failoverMigrateCmd.Flags().StringVar(&migrateMode, "mode", "auto", "mode of the groups: auto or manual")
	failoverMigrateCmd.Flags().StringSliceVar(&migrateElectorate, "electorate", nil, "proxy IDs voting in automatic groups (default: the live proxies)")
	_ = failoverMigrateCmd.MarkFlagRequired("source")
	failoverCmd.AddCommand(failoverPoliciesCmd, failoverEventsCmd, failoverMigrateCmd)
	rootCmd.AddCommand(proxiesCmd, failoverCmd)
}
