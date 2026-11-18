package beamsync

import (
	"io"
	"sync"
)

var chunkBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 8*1024*1024)
		return &buf
	},
}

func copyChunked(dst io.Writer, src io.Reader, chunkSize int) (int64, error) {
	var buf []byte
	if chunkSize == 8*1024*1024 {
		bufPtr := chunkBufferPool.Get().(*[]byte)
		defer chunkBufferPool.Put(bufPtr)
		buf = *bufPtr
	} else {
		buf = make([]byte, chunkSize)
	}
	var total int64
	for {
		n, err := io.ReadFull(src, buf)
		if n > 0 {
			nw, werr := dst.Write(buf[:n])
			total += int64(nw)
			if werr != nil {
				return total, werr
			}
		}
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}
