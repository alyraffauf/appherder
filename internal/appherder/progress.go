package appherder

import "io"

// Progress receives download progress updates. total is -1 when the server
// reports no content length.
type Progress interface {
	Download(name string, received, total int64)
}

type progressReader struct {
	reader   io.Reader
	progress Progress
	name     string
	total    int64
	received int64
}

func newProgressReader(reader io.Reader, progress Progress, name string, total int64) io.Reader {
	if progress == nil {
		return reader
	}
	return &progressReader{reader: reader, progress: progress, name: name, total: total}
}

func (pr *progressReader) Read(buf []byte) (int, error) {
	bytesRead, err := pr.reader.Read(buf)
	pr.received += int64(bytesRead)
	pr.progress.Download(pr.name, pr.received, pr.total)
	return bytesRead, err
}
