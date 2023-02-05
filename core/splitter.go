package core

// function to split a file into chunks
import (
	"bufio"
	"fmt"
	"os"
)

func NewSplitter() {

}

func splitFile(filePath string, chunkSize int) ([][]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("Error opening file: %v", err)
	}
	defer file.Close()

	// Read the file into a buffer
	buf := make([]byte, chunkSize)
	var chunks [][]byte
	for {
		n, err := file.Read(buf)
		if n == 0 {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Error reading file: %v", err)
		}
		chunks = append(chunks, buf[:n])
	}
	return chunks, nil
}

func reassembleFile(filePath string, chunks [][]byte) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Error creating file: %v", err)
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, chunk := range chunks {
		if _, err := w.Write(chunk); err != nil {
			return fmt.Errorf("Error writing to file: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("Error flushing writer: %v", err)
	}
	return nil
}
