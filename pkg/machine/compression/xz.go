package compression

import (
	"bufio"
	"io"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
)

type xzDecompressor struct {
	compressedFilePath string
	compressedFile     *os.File
}

func newXzDecompressor(compressedFilePath string) decompressor {
	return &xzDecompressor{
		compressedFilePath: compressedFilePath,
	}
}

func (d *xzDecompressor) srcFilePath() string {
	return d.compressedFilePath
}

func (d *xzDecompressor) reader() (io.Reader, error) {
	srcFile, err := os.Open(d.compressedFilePath)
	if err != nil {
		return nil, err
	}
	d.compressedFile = srcFile
	return srcFile, nil
}

// Will error out if file without .Xz already exists
// Maybe extracting then renaming is a good idea here..
// depends on Xz: not pre-installed on mac, so it becomes a brew dependency
func (*xzDecompressor) copy(w *os.File, r io.Reader) error {
	var cmd *exec.Cmd
	var read io.Reader

	// Prefer Xz utils for fastest performance, fallback to go xi2 impl
	if _, err := exec.LookPath("xz"); err == nil {
		cmd = exec.Command("xz", "-d", "-c")
		cmd.Stdin = r
		read, err = cmd.StdoutPipe()
		if err != nil {
			return err
		}
		cmd.Stderr = os.Stderr
	} else {
		// This XZ implementation is reliant on buffering. It is also 3x+ slower than XZ utils.
		// Consider replacing with a faster implementation (e.g. xi2) if podman machine is
		// updated with a larger image for the distribution base.
		buf := bufio.NewReader(r)
		read, err = xz.NewReader(buf)
		if err != nil {
			return err
		}
	}

	done := make(chan bool)
	go func() {
		if _, err := io.Copy(w, read); err != nil {
			logrus.Error(err)
		}
		done <- true
	}()

	if cmd != nil {
		err := cmd.Start()
		if err != nil {
			return err
		}
		return cmd.Wait()
	}
	<-done
	return nil
}

func (d *xzDecompressor) close() {
	if err := d.compressedFile.Close(); err != nil {
		logrus.Errorf("Unable to close xz file: %q", err)
	}
}
