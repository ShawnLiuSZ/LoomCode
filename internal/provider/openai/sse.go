package openai

import (
	"bytes"
	"fmt"
	"io"
)

// sseReader SSE 事件读取器
type sseReader struct {
	reader io.Reader
	buf    []byte
}

func newSSEReader(r io.Reader) *sseReader {
	return &sseReader{reader: r, buf: make([]byte, 0, 4096)}
}

// Read 读取下一个 SSE 事件的数据部分
func (s *sseReader) Read() ([]byte, error) {
	tmp := make([]byte, 4096)
	for {
		n, err := s.reader.Read(tmp)
		if n > 0 {
			s.buf = append(s.buf, tmp[:n]...)
		}

		// 查找完整的 SSE 事件（以 \n\n 分隔）
		for {
			idx := bytes.Index(s.buf, []byte("\n\n"))
			if idx == -1 {
				break
			}

			line := s.buf[:idx]
			s.buf = s.buf[idx+2:]

			// 跳过注释行
			if len(line) == 0 {
				continue
			}

			// 提取 data: 前缀
			data, ok := bytes.CutPrefix(line, []byte("data: "))
			if !ok {
				data, ok = bytes.CutPrefix(line, []byte("data:"))
			}
			if ok {
				result := make([]byte, len(data))
				copy(result, data)
				return result, nil
			}
		}

		if err != nil {
			if err == io.EOF && len(s.buf) == 0 {
				return nil, err
			}
			if err != io.EOF {
				return nil, fmt.Errorf("sse read: %w", err)
			}
			// EOF but still have data
			if len(s.buf) > 0 {
				remaining := make([]byte, len(s.buf))
				copy(remaining, s.buf)
				s.buf = s.buf[:0]
				return remaining, nil
			}
			return nil, io.EOF
		}
	}
}
