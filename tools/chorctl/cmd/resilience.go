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
	"github.com/hieunyit/tutti/tools/chorctl/internal/api"
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
chorctl proxies`,
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

func init() {
	failoverCmd.AddCommand(failoverPoliciesCmd, failoverEventsCmd)
	rootCmd.AddCommand(proxiesCmd, failoverCmd)
}
