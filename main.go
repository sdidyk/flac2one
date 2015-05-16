package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
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
var seekMap map[uint64]int
var picture *meta.Block

var tagAlbum, tagArtist, tagDate string
var titles []struct {
	string
	uint64
}
var rf, ro, rcue *os.File
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
	rf, err = ioutil.TempFile(os.TempDir(), "flac2one")
	if err != nil {
		log.Fatalln(err)
		os.Exit(2)
	}
	defer os.Remove(rf.Name())
	defer rf.Close()

	// read files
	md5sum = md5.New()
	seekTable = make([]meta.SeekPoint, 0, 1024)
	seekMap = make(map[uint64]int, 1024)
	titles = make(
		[]struct {
			string
			uint64
		},
		0,
		32,
	)
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

	ro, err = os.Create("result.flac")
	if err != nil {
		log.Fatalln(err)
		os.Exit(2)
	}
	defer ro.Close()

	var b []byte
	// STREAM: header
	_, err = ro.Write([]byte("fLaC"))
	if err != nil {
		os.Exit(2)
	}

	// METADATA_BLOCK_HEADER: streaminfo
	b = make([]byte, 4)
	b[0] = byte(meta.TypeStreamInfo)
	b[3] = 34
	ro.Write(b)

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
	ro.Write(b)

	if len(seekTable) > 0 {
		// METADATA_BLOCK_HEADER: seektable
		b = make([]byte, 4)
		size := (8 + 8 + 2) * len(seekTable)
		b[0] = byte(meta.TypeSeekTable)
		b[1] = byte(size >> 16 & 255)
		b[2] = byte(size >> 8 & 255)
		b[3] = byte(size & 255)
		ro.Write(b)

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
			ro.Write(b)
		}
	}

	if picture != nil {
		// METADATA_BLOCK_HEADER: picture
		b = make([]byte, 4)
		b[0] = byte(meta.TypePicture)
		b[1] = byte(picture.Length >> 16 & 255)
		b[2] = byte(picture.Length >> 8 & 255)
		b[3] = byte(picture.Length & 255)
		ro.Write(b)

		// METADATA_BLOCK_PICTURE
		b = make([]byte, picture.Length)
		picture := picture.Body.(*meta.Picture)
		offset := 0
		encUint32(b[offset:], picture.Type)
		offset += 4
		encUint32(b[offset:], uint32(len(picture.MIME)))
		offset += 4
		copy(b[offset:], picture.MIME)
		offset += len(picture.MIME)
		encUint32(b[offset:], uint32(len(picture.Desc)))
		offset += 4
		copy(b[offset:], picture.Desc)
		offset += len(picture.Desc)
		encUint32(b[offset:], picture.Width)
		offset += 4
		encUint32(b[offset:], picture.Height)
		offset += 4
		encUint32(b[offset:], picture.Depth)
		offset += 4
		encUint32(b[offset:], picture.NPalColors)
		offset += 4
		encUint32(b[offset:], uint32(len(picture.Data)))
		offset += 4
		copy(b[offset:], picture.Data)

		ro.Write(b)
	}

	// METADATA_BLOCK_HEADER: padding
	offset, err := ro.Seek(0, os.SEEK_CUR)
	padding := 256 - (offset+4)&(256-1)
	b = make([]byte, 4)
	b[0] = 1<<7 | byte(meta.TypePadding)
	b[3] = byte(padding)
	ro.Write(b)
	ro.Seek(padding, os.SEEK_CUR)

	// copy frames
	rf.Seek(0, os.SEEK_SET)
	io.Copy(ro, rf)

	rcue, err = os.Create("result.cue")
	if err != nil {
		log.Fatalln(err)
		os.Exit(2)
	}
	defer rcue.Close()

	if tagDate != "" {
		rcue.Write([]byte("REM DATE " + tagDate + "\n"))
	}
	rcue.Write([]byte("PERFORMER \"" + tagArtist + "\"\n"))
	rcue.Write([]byte("TITLE \"" + tagAlbum + "\"\n"))
	rcue.Write([]byte("FILE \"result.flac\" WAVE\n"))
	for i, v := range titles {
		rcue.Write([]byte(fmt.Sprintf("  TRACK %02d AUDIO\n", i+1)))
		rcue.Write([]byte(fmt.Sprintf("    TITLE \"%s\"\n", v.string)))
		rcue.Write([]byte(fmt.Sprintf("    INDEX 01 %s\n", samplesToTime(v.uint64))))
	}
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
		frameSizeMin = 4294967295
		frameSizeMax = 0
		if blockSizeMin != blockSizeMax {
			return fmt.Errorf("not fixed-blocksize; min %v, max %v", stream.Info.BlockSizeMin, stream.Info.BlockSizeMax)
		}
	} else {
		if sampleRate != stream.Info.SampleRate {
			return fmt.Errorf("sample rate mismatch; expected %v, got %v", sampleRate, stream.Info.SampleRate)
		}
		if nChannels != stream.Info.NChannels {
			return fmt.Errorf("num of channels mismatch; expected %v, got %v", nChannels, stream.Info.NChannels)
		}
		if bitsPerSample != stream.Info.BitsPerSample {
			return fmt.Errorf("bits per sample mismatch; expected %v, got %v", bitsPerSample, stream.Info.BitsPerSample)
		}
	}

	// get meta
	titles = append(
		titles,
		struct {
			string
			uint64
		}{"", totalSamples},
	)
	for _, block := range stream.Blocks {
		switch body := block.Body.(type) {
		case *meta.SeekTable:
			for _, point := range body.Points {
				if point.SampleNum != meta.PlaceholderPoint {
					seekMap[point.SampleNum+totalSamples] = len(seekTable)
					seekTable = append(
						seekTable,
						meta.SeekPoint{
							point.SampleNum + totalSamples,
							0,
							point.NSamples,
						},
					)
				}
			}

		case *meta.VorbisComment:
			for _, tag := range body.Tags {
				switch tag[0] {
				case "ALBUM", "Album":
					if first {
						tagAlbum = tag[1]
					}
				case "ARTIST", "Artist":
					if first {
						tagArtist = tag[1]
					}
				case "DATE", "Date":
					if first {
						tagDate = tag[1]
					}
				case "TITLE", "Title":
					titles[len(titles)-1].string = tag[1]
				}
			}

		case *meta.Picture:
			if first && picture == nil && body.Type == 3 {
				picture = block
			}
		}
	}

	// get start offset
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
	samples := uint64(0)
	for {
		var n int
		offset := totalBytes
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
		frame.Hash(md5sum)
		next, err := stream.Pos()
		if err != nil {
			return err
		}
		size := next - start
		frameNumBytes := getUtf8Size(frame.Num)
		newFrameNum := encodeUtf8(frame.Num + totalFrames)

		// copy frame
		// (header)
		b := make([]byte, 4)
		f.Seek(start, os.SEEK_SET)
		f.Read(b)
		if b[1]&1 == 1 {
			return fmt.Errorf("frame hasn't fixed-blocksize")
		}
		rf.Write(b)
		totalBytes += 4
		crcHeader.Write(b)
		crcFrame.Write(b)

		additionalBytes := int64(0)
		// blocksize bits == 011x
		if b[2]&0xE0 == 0x60 {
			additionalBytes += 1
			if b[2]&0x10>>4 != 0 {
				additionalBytes += 1
			}
		}
		// sample rate bits == 11xx
		if b[2]&0x0C == 0x0C {
			additionalBytes += 1
			if b[2]&0x03 != 0 {
				additionalBytes += 1
			}
		}

		// (new frame number)
		n, _ = rf.Write(newFrameNum)
		totalBytes += uint64(n)
		crcHeader.Write(newFrameNum)
		crcFrame.Write(newFrameNum)

		// (additional bytes)
		if additionalBytes > 0 {
			f.Seek(start+4+frameNumBytes, os.SEEK_SET)
			io.CopyN(rf, hr, additionalBytes)
			totalBytes += uint64(additionalBytes)
		}

		// (new crc8)
		crc8 := crcHeader.Sum8()
		n, _ = rf.Write([]byte{crc8})
		totalBytes += 1
		crcFrame.Write([]byte{crc8})

		// (rest of frame)
		restSize := size - (4 + frameNumBytes + additionalBytes + 1) - 2
		f.Seek(start+4+frameNumBytes+additionalBytes+1, os.SEEK_SET)
		io.CopyN(rf, hr, restSize)
		totalBytes += uint64(restSize)

		// (new crc16)
		crc16 := crcFrame.Sum16()
		rf.Write([]byte{byte(crc16 >> 8), byte(crc16 & 0xff)})
		totalBytes += 2

		// recalculate seektable offset
		sampleNum := samples + totalSamples
		if i, ok := seekMap[sampleNum]; ok {
			seekTable[i].Offset = offset
		}
		// recalculate new frame size
		if frameNumBytes < int64(len(newFrameNum)) {
			size += int64(len(newFrameNum)) - frameNumBytes
		}
		// update min and max
		if uint32(size) < frameSizeMin {
			frameSizeMin = uint32(size)
		}
		if uint32(size) > frameSizeMax {
			frameSizeMax = uint32(size)
		}
		if frame.BlockSize < blockSizeMin {
			blockSizeMin = frame.BlockSize
		}
		if frame.BlockSize > blockSizeMax {
			blockSizeMax = frame.BlockSize
		}
		// next iteration
		start = next
		frames += 1
		samples += uint64(frame.BlockSize)
	}

	// update totals
	totalSamples += samples
	totalFrames += frames

	return nil
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

func encUint32(b []byte, n uint32) {
	b[0] = byte(n >> 24 & 255)
	b[1] = byte(n >> 16 & 255)
	b[2] = byte(n >> 8 & 255)
	b[3] = byte(n & 255)
	return
}

func samplesToTime(n uint64) string {
	t := n * 75 / uint64(sampleRate)
	m := t / (60 * 75)
	s := (t - m*60*75) / 75
	f := t % 75
	return fmt.Sprintf("%02d:%02d:%02d", m, s, f)
}
