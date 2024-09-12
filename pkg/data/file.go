package data

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"time"
)

type DataFile struct {
	file *os.File
}

func NewDataFile(fileName string) (*DataFile, error) {
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0777)
	if err != nil {
		return nil, err
	}

	return &DataFile{
		file: file,
	}, nil
}

func (b *DataFile) Write(data []byte) (int, error) {
	return b.file.Write(data)
}

func (b *DataFile) Close() error {
	return b.file.Close()
}

const (
	SlotsPerFile = 1000
)

type timeTrackedFile struct {
	lastAccess time.Time
	file       *DataFile
}

type FileManager struct {
	directory string

	starter    sync.Once
	mu         sync.RWMutex
	fileLookup map[uint64]string
	openFiles  map[string]*timeTrackedFile
}

func NewFileManager(directory string) *FileManager {
	return &FileManager{
		directory:  directory,
		fileLookup: make(map[uint64]string),
		openFiles:  make(map[string]*timeTrackedFile),
	}
}

func (w *FileManager) Start(ctx context.Context) error {
	w.starter.Do(func() {
		go func(ctx context.Context) {
			ticker := time.NewTicker(10 * time.Second)
			for {
				select {
				case <-ctx.Done():
				case <-ticker.C:
					w.mu.Lock()

					for key, value := range w.openFiles {
						if time.Since(value.lastAccess) >= 2*time.Minute {
							_ = value.file.Close()
							delete(w.openFiles, key)
						}
					}

					w.mu.Unlock()
				}
			}
		}(ctx)
	})

	return nil
}

func (w *FileManager) WriterForSlot(slot uint64) (io.Writer, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	baseSlot := uint64(math.Floor(float64(slot)/float64(SlotsPerFile)) * SlotsPerFile)

	fileName, ok := w.fileLookup[baseSlot]
	if !ok {
		fileName = fmt.Sprintf("%s/logs_slot_%d.txt", w.directory, baseSlot)
		w.fileLookup[baseSlot] = fileName
	}

	dataFile, ok := w.openFiles[fileName]
	if !ok {
		file, err := NewDataFile(fileName)
		if err != nil {
			return nil, err
		}

		dataFile = &timeTrackedFile{
			file: file,
		}

		w.openFiles[fileName] = dataFile
	}

	dataFile.lastAccess = time.Now()

	return dataFile.file, nil
}
