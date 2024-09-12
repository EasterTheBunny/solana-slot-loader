package data

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"

	"github.com/easterthebunny/solana-slot-loader/pkg/async"
)

type Loader interface {
	LoadBlockMessages(ctx context.Context, slot uint64) ([]InstructionLogs, error)
}

type WriteManager interface {
	WriterForSlot(uint64) (io.Writer, error)
}

type SlotPreProcessor struct {
	writer    WriteManager
	loader    Loader
	group     *async.WorkerGroup
	directory string
	chSlots   <-chan uint64
}

func NewSlotRangeService(writer WriteManager, loader Loader, group *async.WorkerGroup, directory string, slots <-chan uint64) *SlotPreProcessor {
	return &SlotPreProcessor{
		writer:    writer,
		loader:    loader,
		group:     group,
		directory: directory,
		chSlots:   slots,
	}
}

func (p *SlotPreProcessor) Process(ctx context.Context) error {
	for {
		select {
		case slot := <-p.chSlots:
			data, err := p.writer.WriterForSlot(slot)
			if err != nil {
				log.Printf("error getting writer: %s", err.Error())
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
		case <-ctx.Done():
			return nil
		}
	}
}
