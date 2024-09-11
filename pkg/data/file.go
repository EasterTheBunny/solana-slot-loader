package data

import (
	"os"
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
