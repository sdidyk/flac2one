package flac

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/mewkiz/flac/frame"
	"github.com/mewkiz/flac/meta"
)

type Stream struct {
	Info   *meta.StreamInfo
	Blocks []*meta.Block
	r      *bufio.Reader
	f      *os.File
}

var signature = []byte("fLaC")

func (stream *Stream) parseStreamInfo() (isLast bool, err error) {
	r := stream.r
	var buf [4]byte
	_, err = io.ReadFull(r, buf[:])
	if err != nil {
		return false, err
	}
	if !bytes.Equal(buf[:], signature) {
		return false, fmt.Errorf("flac.parseStreamInfo: invalid FLAC signature; expected %q, got %q", signature, buf)
	}

	block, err := meta.Parse(r)
	if err != nil {
		return false, err
	}
	si, ok := block.Body.(*meta.StreamInfo)
	if !ok {
		return false, fmt.Errorf("flac.parseStreamInfo: incorrect type of first metadata block; expected *meta.StreamInfo, got %T", si)
	}
	stream.Info = si
	return block.IsLast, nil
}

func Parse(r io.Reader) (stream *Stream, err error) {
	br := bufio.NewReader(r)
	stream = &Stream{r: br}
	isLast, err := stream.parseStreamInfo()
	if err != nil {
		return nil, err
	}

	for !isLast {
		block, err := meta.Parse(br)
		if err != nil {
			if err != meta.ErrReservedType {
				return stream, err
			}
			err = block.Skip()
			if err != nil {
				return stream, err
			}
		}
		stream.Blocks = append(stream.Blocks, block)
		isLast = block.IsLast
	}

	return stream, nil
}

func ParseFile(path string) (stream *Stream, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	stream, err = Parse(f)
	stream.f = f
	return stream, err
}

func (stream *Stream) Close() error {
	return stream.f.Close()
}

func (stream *Stream) Next() (f *frame.Frame, err error) {
	return frame.New(stream.r)
}

func (stream *Stream) ParseNext() (f *frame.Frame, err error) {
	return frame.Parse(stream.r)
}

func (stream *Stream) Pos() (pos int64, err error) {
	pos, err = stream.f.Seek(0, os.SEEK_CUR)
	pos -= int64(stream.r.Buffered())
	return
}
