package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/easterthebunny/solana-slot-loader/pkg/async"
	"github.com/easterthebunny/solana-slot-loader/pkg/data"
	"github.com/easterthebunny/solana-slot-loader/pkg/rpc"
)

func init() {
	rootCmd.Flags().StringVarP(&network, "network", "n", "private", "network configuration")
	rootCmd.Flags().StringVar(&rpcEndpoint, "rpc", rpc.DevNet_RPC, "rpc endpoint (devnet default)")
	rootCmd.Flags().StringVar(&wsEndpoint, "ws", rpc.DevNet_WS, "websocket endpoint (devnet default)")
	rootCmd.Flags().Uint64Var(&backfillRange, "backfill-to", 50_000, "number of slots to backfill to (default 50,000)")
}

var (
	network       string
	rpcEndpoint   string
	wsEndpoint    string
	backfillRange uint64

	rootCmd = &cobra.Command{
		Short: "run",
		Long:  "run",
		Run: func(cmd *cobra.Command, _ []string) {
			loader, err := rpc.NewLoader(getLoaderConfig(cmd))
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s", err.Error())
				os.Exit(2)
			}

			ctx, cancel := context.WithCancel(cmd.Context())

			chSig := make(chan os.Signal, 1)
			signal.Notify(chSig, os.Interrupt)
			go func() {
				for range chSig {
					cancel()
				}
			}()

			chSlots := make(chan uint64, 100)

			slotLoadSvc := rpc.NewSlotRangeService(loader, chSlots)
			_ = slotLoadSvc.LoadAndBackfill(ctx, backfillRange)

			group := async.NewWorkerGroup(20)

			slotRangeSvc := data.NewSlotRangeService(loader, group, chSlots)
			if err := slotRangeSvc.Process(ctx); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
				os.Exit(3)
			}

			group.Stop()
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getLoaderConfig(cmd *cobra.Command) rpc.LoaderConfig {
	switch network {
	case "dev":
		rpcEndpoint = rpc.DevNet_RPC
		wsEndpoint = rpc.DevNet_WS
	case "main_beta":
		rpcEndpoint = rpc.MainNetBeta_RPC
		wsEndpoint = rpc.MainNetBeta_WS
	case "private":
		break
	default:
		fmt.Fprintln(cmd.ErrOrStderr(), "invalid network")
		os.Exit(3)
	}

	return rpc.LoaderConfig{
		RPC: rpcEndpoint,
		WS:  wsEndpoint,
	}
}
