package rpc

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
	"github.com/gagliardetto/solana-go/rpc/ws"

	"github.com/easterthebunny/solana-slot-loader/pkg/data"
)

const (
	DevNet_RPC = rpc.DevNet_RPC
	DevNet_WS  = rpc.DevNet_WS

	MainNetBeta_RPC = rpc.MainNetBeta_RPC
	MainNetBeta_WS  = rpc.MainNetBeta_WS
)

var (
	ErrContextCancelled = errors.New("context cancelled")
)

type Loader struct {
	client   *rpc.Client
	wsClient *ws.Client
}

type LoaderConfig struct {
	RPC string
	WS  string
}

func NewLoader(config LoaderConfig) (*Loader, error) {
	wsClient, err := ws.Connect(context.Background(), config.WS)
	if err != nil {
		return nil, err
	}

	return &Loader{
		client:   rpc.New(config.RPC),
		wsClient: wsClient,
	}, nil
}

func (l *Loader) SlotSubscribe(ctx context.Context, chSlot chan uint64) error {
	sub, err := l.wsClient.SlotSubscribe()
	if err != nil {
		return err
	}

	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return ErrContextCancelled
		default:
			slot, err := sub.Recv()
			if err != nil {
				return err
			}

			chSlot <- slot.Slot
		}
	}
}

func (l *Loader) GetLatestSlot(ctx context.Context) (uint64, error) {
	result, err := l.client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}

	return result.Context.Slot, nil
}

func (l *Loader) LoadSlotsRange(ctx context.Context, start, end uint64, chSlots chan<- uint64) error {
	if start >= end {
		return errors.New("the start block must come before the end block")
	}

	result, err := l.client.GetBlocks(ctx, start, &end, rpc.CommitmentFinalized)
	if err != nil {
		return err
	}

	for _, slot := range result {
		chSlots <- slot
	}

	return nil
}

func (l *Loader) LoadSlotsFrom(ctx context.Context, start uint64, chSlots chan<- uint64) (uint64, error) {
	var lastLoaded uint64

	end, err := l.GetLatestSlot(ctx)
	if err != nil {
		return 0, err
	}

	if start == 0 {
		start = end - 20
	} else {
		start++
	}

	if start > end {
		return 0, nil
	}

	result, err := l.client.GetBlocks(ctx, start, &end, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}

	for _, slot := range result {
		if slot > end {
			lastLoaded = slot
		}

		chSlots <- slot
	}

	if lastLoaded == 0 {
		lastLoaded = start
	}

	return lastLoaded, nil
}

func (l *Loader) LoadBlockMessages(ctx context.Context, slot uint64) ([]data.InstructionLogs, error) {
	logs := make([]data.InstructionLogs, 0)

	includeRewards := false
	block, err := l.client.GetBlockWithOpts(
		ctx,
		slot,
		// You can specify more options here:
		&rpc.GetBlockOpts{
			Encoding:   solana.EncodingBase64,
			Commitment: rpc.CommitmentFinalized,
			// Get only signatures:
			TransactionDetails: rpc.TransactionDetailsSignatures,
			// Exclude rewards:
			Rewards: &includeRewards,
		},
	)
	if err != nil {
		return nil, err
	}

	opts := rpc.GetTransactionOpts{
		Encoding:   solana.EncodingBase64,
		Commitment: rpc.CommitmentFinalized,
	}

	for _, sig := range block.Signatures {
		trx, err := l.client.GetTransaction(ctx, sig, &opts)
		if err != nil {
			if err, isRPCError := err.(*jsonrpc.RPCError); isRPCError && err.Code == -32015 {
				continue
			}

			log.Printf("%s\n", err.Error())

			continue
		}

		logs = append(logs, data.ParseLogs(trx.Meta.LogMessages)...)
	}

	return logs, nil
}

type SlotRangeService struct {
	chSlot     chan<- uint64
	loader     *Loader
	start, end uint64

	starter sync.Once
	running atomic.Bool
	chClose chan struct{}
}

func NewSlotRangeService(loader *Loader, slots chan<- uint64) *SlotRangeService {
	return &SlotRangeService{
		chSlot:  slots,
		loader:  loader,
		chClose: make(chan struct{}, 1),
	}
}

func (s *SlotRangeService) LoadAndBackfill(ctx context.Context, backfillRange uint64) error {
	s.starter.Do(func() {
		s.running.Store(true)

		slot, err := s.loader.GetLatestSlot(ctx)
		if err != nil {
			log.Printf("error loading recent slot: %s", err.Error())

			return
		}

		s.end = slot
		s.start = slot - backfillRange

		if s.start > s.end {
			log.Printf("backfill range overflow")

			return
		}

		go s.runFrom(ctx, s.loader, s.chSlot)
		go s.runRange(ctx, s.loader, s.chSlot)
	})

	return nil
}

func (s *SlotRangeService) LoadRange(ctx context.Context, start, end uint64) error {
	s.starter.Do(func() {
		s.running.Store(true)

		go s.runRange(ctx, s.loader, s.chSlot)
	})

	return nil
}

func (s *SlotRangeService) Close() error {
	if s.running.Load() {
		s.chClose <- struct{}{}
	}

	return nil
}

func (s *SlotRangeService) runFrom(ctx context.Context, loader *Loader, chSlot chan<- uint64) {
	ticker := time.NewTicker(1 * time.Second)
	last := s.end

	var err error

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.chClose:
			return
		case <-ticker.C:
			if last, err = loader.LoadSlotsFrom(ctx, last, chSlot); err != nil {
				log.Printf("slot subscriber error: %s", err.Error())
			}
		}
	}
}

func (s *SlotRangeService) runRange(ctx context.Context, loader *Loader, chSlot chan<- uint64) {
	start := s.start
	end := s.end

	if end-start > 10_000 {
		start = end - 10_000
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.chClose:
			return
		default:
			if err := loader.LoadSlotsRange(ctx, start, end, chSlot); err != nil {
				log.Printf("slot subscriber error: %s", err.Error())
			}

			end = start
			start = start - 10_000

			if start < s.start {
				start = s.start
			}

			if end <= start {
				return
			}
		}
	}
}
