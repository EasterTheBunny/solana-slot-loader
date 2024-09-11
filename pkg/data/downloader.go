package data

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/easterthebunny/solana-slot-loader/pkg/async"
)

const (
	BlocksPerFile = 50
)

type Loader interface {
	LoadBlockMessages(ctx context.Context, slot uint64) ([]InstructionLogs, error)
}

type SlotPreProcessor struct {
	loader  Loader
	group   *async.WorkerGroup
	chSlots <-chan uint64
}

func NewSlotRangeService(loader Loader, group *async.WorkerGroup, slots <-chan uint64) *SlotPreProcessor {
	return &SlotPreProcessor{
		loader:  loader,
		group:   group,
		chSlots: slots,
	}
}

func (p *SlotPreProcessor) Process(ctx context.Context) error {
	var (
		data      *DataFile
		startSlot uint64
		slotCount int
		err       error
	)

	defer func() {
		if data != nil {
			data.Close()
		}
	}()

	for {
		select {
		case slot := <-p.chSlots:
			if data == nil || slotCount > BlocksPerFile {
				if data != nil {
					data.Close()
				}

				startSlot = slot
				slotCount = 0

				if data, err = NewDataFile(fmt.Sprintf("./data_files/slot_logs_%d.txt", startSlot)); err != nil {
					return err
				}
			}

			if err = p.group.Do(ctx, func(ctx context.Context) {
				logs, err := p.loader.LoadBlockMessages(context.Background(), slot)
				if err != nil {
					log.Printf("LoadBlockMessages err: %s", err.Error())
				}

				for _, log := range logs {
					for _, message := range log.Logs {
						var buf bytes.Buffer
						buf.WriteString(fmt.Sprintf("%d::%s::%s\n", slot, log.Program, message.Text))

						_, _ = data.Write(buf.Bytes())
					}
				}
			}); err != nil {
				return err
			}

			slotCount++

		case <-ctx.Done():
			return nil
		}
	}
}
