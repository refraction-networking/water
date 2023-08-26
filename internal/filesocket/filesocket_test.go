package filesocket

import (
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

func TestFileSocket(t *testing.T) {
	t.Run("testTempFileSocket", testTempFileSocket)
}

func testTempFileSocket(t *testing.T) {
	// Create 2 temp files
	rdFile, err := os.CreateTemp("", "rdFile_*.tmp")
	if err != nil {
		t.Fatalf("error in creating temp rdFile: %v", err)
	}
	t.Logf("created: %v", rdFile.Name())
	// close and remove the file on exit
	defer func() {
		rdFile.Close()
		os.Remove(rdFile.Name())
	}()

	wrFile, err := os.CreateTemp("", "wrFile_*.tmp")
	if err != nil {
		t.Fatalf("error in creating temp wrFile: %v", err)
	}
	t.Logf("created: %v", wrFile.Name())
	// close and remove the file on exit
	defer func() {
		wrFile.Close()
		os.Remove(wrFile.Name())
	}()

	// Create a FileSocket
	fs := NewFileSocket(rdFile, wrFile)
	if fs == nil {
		t.Fatalf("NewFileSocket() returned nil")
	}
	defer fs.Close()

	wg := &sync.WaitGroup{}
	wg.Add(3) // 3 goroutines

	// One goroutine to read from the wrFile, capitalise the string and write to rdFile

	wrFileCopy, err := os.OpenFile(wrFile.Name(), os.O_RDONLY, 0644)
	if err != nil {
		t.Errorf("error in opening wrFile: %v", err)
		return
	}

	rdFileCopy, err := os.OpenFile(rdFile.Name(), os.O_WRONLY, 0644)
	if err != nil {
		t.Errorf("error in opening rdFile: %v", err)
		return
	}

	go func(wrFile, rdFile *os.File) {
		defer wg.Done()
		var buf []byte = make([]byte, 1024)
		for {
			n, err := wrFile.Read(buf)
			if err != nil && !errors.Is(err, io.EOF) {
				if !errors.Is(err, os.ErrClosed) {
					t.Errorf("wrFile.Read() errored: %v", err)
				}
				return
			}
			if n == 0 {
				// t.Logf("wrFile.Read(): empty read")
				continue
			}
			// t.Logf("wrFile.Read(): %v bytes read: %v", n, string(buf[:n]))

			// Capitalise the string
			for i := 0; i < n; i++ {
				if buf[i] >= 'a' && buf[i] <= 'z' {
					buf[i] = buf[i] - 'a' + 'A'
				}
			}

			// Write to rdFile
			n, err = rdFile.Write(buf[:n])
			if err != nil {
				t.Logf("rdFile.Write() errored: %v", err)
				return
			}
			// t.Logf("rdFile.Write(): %v bytes written", n)

			// Sleep for 10 millisecond
			time.Sleep(10 * time.Millisecond)
		}
	}(wrFileCopy, rdFileCopy)

	// one goroutine used to write to the FileSocket
	go func() {
		defer wg.Done()
		defer fs.Close()
		defer wrFileCopy.Close()
		for i := 0; i < 10; i++ {
			// Write to the FileSocket
			sendBuf := []byte("hello world")
			n, err := fs.Write(sendBuf)
			if err != nil {
				t.Errorf("fs.Write() errored: %v", err)
				return
			}

			if n != len(sendBuf) {
				t.Errorf("fs.Write() wrote %v bytes, expected %v bytes", n, len(sendBuf))
				return
			}
			t.Logf("fs.Write(): %v bytes written: %v", n, string(sendBuf))

			// Sleep for 1 Second
			time.Sleep(1 * time.Second)
		}
	}()

	// one goroutine used to read from the FileSocket
	go func() {
		defer wg.Done()
		defer rdFileCopy.Close()
		for {
			buf := make([]byte, 1024)
			n, err := fs.Read(buf)
			if err != nil {
				if errors.Is(err, os.ErrClosed) && fs.(*fileSocket).closed.Load() {
					t.Logf("fs.Read(): reading from closed socket")
					return
				}
				if errors.Is(err, io.EOF) {
					t.Logf("fs.Read(): EOF")
					return
				}
				t.Errorf("fs.Read() errored: %v", err)
				return
			}
			if n == 0 {
				// t.Logf("fs.Read(): empty read")
				continue
			}
			t.Logf("fs.Read(): %v bytes read: %v", n, string(buf[:n]))
		}
	}()

	// Wait for the goroutines to finish
	wg.Wait()
}
