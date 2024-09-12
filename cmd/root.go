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
	rootCmd.Flags().StringVarP(&directory, "output", "o", "./data_files", "log file output directory")
	rootCmd.Flags().IntVarP(&workerCount, "threads", "t", 20, "worker thread count")

	rootCmd.Flags().StringVar(&rpcEndpoint, "rpc", rpc.DevNet_RPC, "rpc endpoint (devnet default)")
	rootCmd.Flags().StringVar(&wsEndpoint, "ws", rpc.DevNet_WS, "websocket endpoint (devnet default)")
	rootCmd.Flags().Uint8Var(&limit, "rpc-limit", 0, "query per second limit on rpc (default 0 -- no limit)")

	rootCmd.Flags().BoolVar(&backfill, "backfill", false, "should the service backfill")
	rootCmd.Flags().Uint64Var(&backfillRange, "backfill-depth", 10_000, "number of slots to backfill to (default 50,000)")
}

var (
	network     string
	directory   string
	workerCount int

	rpcEndpoint string
	wsEndpoint  string
	limit       uint8

	backfill      bool
	backfillRange uint64

	rootCmd = &cobra.Command{
		Short: "run",
		Long:  "run",
		Run: func(cmd *cobra.Command, _ []string) {
			ctx, cancel := context.WithCancel(cmd.Context())

			loader, err := rpc.NewLoader(getLoaderConfig(ctx, cmd))
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s", err.Error())
				os.Exit(2)
			}

			chSig := make(chan os.Signal, 1)
			signal.Notify(chSig, os.Interrupt)
			go func() {
				for range chSig {
					cancel()
				}
			}()

			chSlots := make(chan uint64, 1000)

			if backfill {
				slotLoadSvc := rpc.NewSlotRangeService(loader, chSlots)
				_ = slotLoadSvc.LoadAndBackfill(ctx, backfillRange)
			}

			group := async.NewWorkerGroup(workerCount)
			writeManager := data.NewFileManager(directory)
			_ = writeManager.Start(ctx)

			slotRangeSvc := data.NewSlotRangeService(writeManager, loader, group, directory, chSlots)
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

func getLoaderConfig(ctx context.Context, cmd *cobra.Command) rpc.LoaderConfig {
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

	var limiter *async.RateLimiter

	if limit > 0 {
		limiter = async.NewRateLimiter(limit)
		limiter.Start(ctx)
	}

	return rpc.LoaderConfig{
		RPC:     rpcEndpoint,
		WS:      wsEndpoint,
		Limiter: limiter,
	}
}
