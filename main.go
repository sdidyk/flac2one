package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"hash"
	"io"
	"log"
	"os"

	"github.com/sdidyk/flac2one/flac"
	"gopkg.in/mewkiz/flac.v1/meta"
	"gopkg.in/mewpkg/hashutil.v1/crc16"
	"gopkg.in/mewpkg/hashutil.v1/crc8"
)

func init() {
	flag.Usage = usage
}

func usage() {
	fmt.Println("Usage: flac2one [options] <flac files>")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
}

var first bool
var blockSizeMin, blockSizeMax uint16
var frameSizeMin, frameSizeMax uint32
var sampleRate uint32
var nChannels, bitsPerSample uint8
var totalBytes, totalSamples, totalFrames uint64
var seekTable []meta.SeekPoint

var rf *os.File
var md5sum hash.Hash

func main() {
	var err error

	// flag parse and usage
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// create output file
	rf, err = os.Create("result.flac")
	if err != nil {
		os.Exit(2)
	}
	// padding for meta
	rf.Seek(1024*64, os.SEEK_SET)

	// read files
	md5sum = md5.New()
	seekTable = make([]meta.SeekPoint, 0, 1024)
	first = true
	for _, path := range flag.Args() {
		fmt.Println(path)
		err := list(path)
		if err != nil {
			log.Fatalln(err)
			os.Exit(3)
		}
		first = false
	}
	rf.Seek(0, os.SEEK_SET)

	var b []byte
	// STREAM: header
	_, err = rf.Write([]byte("fLaC"))
	if err != nil {
		os.Exit(2)
	}

	// METADATA_BLOCK_HEADER: streaminfo
	b = make([]byte, 4)
	b[0] = 0
	b[3] = 34
	rf.Write(b)

	// METADATA_BLOCK_STREAMINFO
	b = make([]byte, 34)
	b[0] = byte(blockSizeMin >> 8 & 255)
	b[1] = byte(blockSizeMin & 255)
	b[2] = byte(blockSizeMax >> 8 & 255)
	b[3] = byte(blockSizeMax & 255)
	b[4] = byte(frameSizeMin >> 16 & 255)
	b[5] = byte(frameSizeMin >> 8 & 255)
	b[6] = byte(frameSizeMin & 255)
	b[7] = byte(frameSizeMax >> 16 & 255)
	b[8] = byte(frameSizeMax >> 8 & 255)
	b[9] = byte(frameSizeMax & 255)
	b[10] = byte(sampleRate >> 12 & 255)
	b[11] = byte(sampleRate >> 4 & 255)
	b[12] = byte(sampleRate&15<<4) | byte((nChannels-1)<<1) | byte((bitsPerSample-1)>>4)
	b[13] = byte((bitsPerSample-1)&15<<4) | byte(totalSamples>>32&255)
	b[14] = byte(totalSamples >> 24 & 255)
	b[15] = byte(totalSamples >> 16 & 255)
	b[16] = byte(totalSamples >> 8 & 255)
	b[17] = byte(totalSamples & 255)
	copy(b[18:], md5sum.Sum(nil))
	rf.Write(b)

	if false && len(seekTable) > 0 {
		// METADATA_BLOCK_HEADER: seektable
		b = make([]byte, 4)
		size := (8 + 8 + 2) * len(seekTable)
		b[0] = 3
		b[1] = byte(size >> 16 & 255)
		b[2] = byte(size >> 8 & 255)
		b[3] = byte(size & 255)
		rf.Write(b)

		// METADATA_BLOCK_SEEKTABLE
		b = make([]byte, 8+8+2)
		for _, v := range seekTable {
			b[0] = byte(v.SampleNum >> 56 & 255)
			b[1] = byte(v.SampleNum >> 48 & 255)
			b[2] = byte(v.SampleNum >> 40 & 255)
			b[3] = byte(v.SampleNum >> 32 & 255)
			b[4] = byte(v.SampleNum >> 24 & 255)
			b[5] = byte(v.SampleNum >> 16 & 255)
			b[6] = byte(v.SampleNum >> 8 & 255)
			b[7] = byte(v.SampleNum & 255)
			b[8] = byte(v.Offset >> 56 & 255)
			b[9] = byte(v.Offset >> 48 & 255)
			b[10] = byte(v.Offset >> 40 & 255)
			b[11] = byte(v.Offset >> 32 & 255)
			b[12] = byte(v.Offset >> 24 & 255)
			b[13] = byte(v.Offset >> 16 & 255)
			b[14] = byte(v.Offset >> 8 & 255)
			b[15] = byte(v.Offset & 255)
			b[16] = byte(v.NSamples >> 8 & 255)
			b[17] = byte(v.NSamples & 255)
			rf.Write(b)
		}
	}

	// METADATA_BLOCK_HEADER: padding
	offset, err := rf.Seek(0, os.SEEK_CUR)
	b = make([]byte, 4)
	padding := 1024*64 - offset - 4
	b[0] = 1<<7 | 1
	b[1] = byte(padding >> 16 & 255)
	b[2] = byte(padding >> 8 & 255)
	b[3] = byte(padding & 255)
	rf.Write(b)

	// METADATA_BLOCK_PADDING
	// (already)

	rf.Close()
}

func list(path string) (err error) {
	// open file
	stream, err := flac.ParseFile(path)
	if err != nil {
		return err
	}
	defer stream.Close()

	// check info
	if first {
		sampleRate = stream.Info.SampleRate
		nChannels = stream.Info.NChannels
		bitsPerSample = stream.Info.BitsPerSample
		blockSizeMin = stream.Info.BlockSizeMin
		blockSizeMax = stream.Info.BlockSizeMax
		frameSizeMin = stream.Info.FrameSizeMin
		frameSizeMax = stream.Info.FrameSizeMax
		if blockSizeMin != blockSizeMax {
			return fmt.Errorf("flac2cue: not fixed-blocksize; min %v, max %v", stream.Info.BlockSizeMin, stream.Info.BlockSizeMax)
		}
	} else {
		if sampleRate != stream.Info.SampleRate {
			return fmt.Errorf("flac2cue: sample rate mismatch; expected %v, got %v", sampleRate, stream.Info.SampleRate)
		}
		if nChannels != stream.Info.NChannels {
			return fmt.Errorf("flac2cue: num of channels mismatch; expected %v, got %v", nChannels, stream.Info.NChannels)
		}
		if bitsPerSample != stream.Info.BitsPerSample {
			return fmt.Errorf("flac2cue: bits per sample mismatch; expected %v, got %v", bitsPerSample, stream.Info.BitsPerSample)
		}
		if blockSizeMin != stream.Info.BlockSizeMin {
			return fmt.Errorf("flac2cue: min blocksize mismatch; expected %v, got %v", blockSizeMin, stream.Info.BlockSizeMin)
		}
		if blockSizeMax != stream.Info.BlockSizeMax {
			return fmt.Errorf("flac2cue: max blocksize mismatch; expected %v, got %v", blockSizeMax, stream.Info.BlockSizeMax)
		}
		if stream.Info.FrameSizeMin < frameSizeMin {
			frameSizeMin = stream.Info.FrameSizeMin
		}
		if stream.Info.FrameSizeMax > frameSizeMax {
			frameSizeMax = stream.Info.FrameSizeMax
		}
	}

	// get meta
	for blockNum, block := range stream.Blocks {
		listBlock(block, blockNum+1)
	}

	totalSamples += stream.Info.NSamples
	start, err := stream.Pos()
	if err != nil {
		return err
	}

	// reopen file for copying
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// rewrite frames
	frames := uint64(0)
	for {
		var n int
		crcHeader := crc8.NewATM()
		crcFrame := crc16.NewIBM()

		fr := io.TeeReader(f, crcHeader)
		hr := io.TeeReader(fr, crcFrame)

		frame, err := stream.ParseNext()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		// wait for https://github.com/mewkiz/flac/issues/8
		if false && frame.HasFixedBlockSize != true {
			return fmt.Errorf("flac2cue: frame hasn't fixed-blocksize")
		}
		frame.Hash(md5sum)
		next, err := stream.Pos()
		if err != nil {
			return err
		}
		size := next - start
		frameNumBytes := getUtf8Size(frame.Num)
		newFrameNum := encodeUtf8(frame.Num + totalFrames)
		// fmt.Println(start, size, frame.Num*uint64(blockSizeMin))

		// copy frame
		// (header)
		f.Seek(start, os.SEEK_SET)
		io.CopyN(rf, hr, 4)
		totalBytes += uint64(4)
		// (new frame number)
		n, _ = rf.Write(newFrameNum)
		totalBytes += uint64(n)
		crcFrame.Write(newFrameNum)
		// (rest of frame)
		restSize := size - (frameNumBytes + 4) - 2
		f.Seek(start+frameNumBytes+4, os.SEEK_SET)
		io.CopyN(rf, hr, restSize)
		totalBytes += uint64(restSize)
		// (new crc16)
		crc16 := crcFrame.Sum16()
		n, _ = rf.Write([]byte{byte(crc16 >> 8), byte(crc16 & 0xff)})
		totalBytes += uint64(n)

		// next iteration
		start = next
		frames++
	}
	totalFrames += frames

	return nil
}

func listBlock(block *meta.Block, blockNum int) {
	switch block.Body.(type) {
	case *meta.SeekTable:
		// for _, point := range body.Points {
		// 	if point.SampleNum != meta.PlaceholderPoint {
		// 		seekTable = append(
		// 			seekTable,
		// 			meta.SeekPoint{
		// 				point.SampleNum + totalSamples,
		// 				point.Offset + totalBytes,
		// 				point.NSamples,
		// 			},
		// 		)
		// 	}
		// }

	case *meta.VorbisComment:
		// for tagNum, tag := range body.Tags {
		// 	fmt.Printf("    comment[%d]: %s=%s\n", tagNum, tag[0], tag[1])
		// }

	case *meta.Picture:
		// listPicture(body)
	}
}

func getUtf8Size(n uint64) (s int64) {
	if n <= 1<<7-1 {
		s = 1
	} else if n <= 1<<11-1 {
		s = 2
	} else if n <= 1<<16-1 {
		s = 3
	} else if n <= 1<<21-1 {
		s = 4
	} else if n <= 1<<26-1 {
		s = 5
	} else if n <= 1<<31-1 {
		s = 6
	} else {
		s = 7
	}
	return
}

func encodeUtf8(n uint64) []byte {
	b := make([]byte, 7)
	if n <= 1<<7-1 {
		b[0] = byte(n & 0x7F)
		return b[:1]

	} else if n <= 1<<11-1 {
		b[0] = byte(n>>6&0x1F | 0xC0)
		b[1] = byte(n&0x3F | 0x80)
		return b[:2]

	} else if n <= 1<<16-1 {
		b[0] = byte(n>>12&0x0F | 0xE0)
		b[1] = byte(n>>6&0x3F | 0x80)
		b[2] = byte(n&0x3F | 0x80)
		return b[:3]

	} else if n <= 1<<21-1 {
		b[0] = byte(n>>18&0x07 | 0xF0)
		b[1] = byte(n>>12&0x3F | 0x80)
		b[2] = byte(n>>6&0x3F | 0x80)
		b[3] = byte(n&0x3F | 0x80)
		return b[:4]

	} else if n <= 1<<26-1 {
		b[0] = byte(n>>24&0x03 | 0xF8)
		b[1] = byte(n>>18&0x3F | 0x80)
		b[2] = byte(n>>12&0x3F | 0x80)
		b[3] = byte(n>>6&0x3F | 0x80)
		b[4] = byte(n&0x3F | 0x80)
		return b[:5]

	} else if n <= 1<<31-1 {
		b[0] = byte(n>>30&0x01 | 0xFC)
		b[1] = byte(n>>24&0x3F | 0x80)
		b[2] = byte(n>>18&0x3F | 0x80)
		b[3] = byte(n>>12&0x3F | 0x80)
		b[4] = byte(n>>6&0x3F | 0x80)
		b[5] = byte(n&0x3F | 0x80)
		return b[:6]

	} else {
		b[0] = byte(0xFE)
		b[1] = byte(n>>30&0x3F | 0x80)
		b[2] = byte(n>>24&0x3F | 0x80)
		b[3] = byte(n>>18&0x3F | 0x80)
		b[4] = byte(n>>12&0x3F | 0x80)
		b[5] = byte(n>>6&0x3F | 0x80)
		b[6] = byte(n&0x3F | 0x80)
		return b[:7]

	}

}
