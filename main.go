package main

import (
	"os"
	"bytes"
	"compress/flate"
	"io/ioutil"
	"encoding/binary"
	"time"
	"fmt"
)

type compression uint8
const (
	noCompression compression = iota
	deflateCompression
)

type localFileHeader struct {
	signature uint32
	version uint16
	bitFlag uint16
	compression compression
	lastModified time.Time
	crc32 uint32
	compressedSize uint32
	uncompressedSize uint32
	fileName string
	extraField []byte
	fileContents string
}

var errOverranBuffer = fmt.Errorf("Overran buffer")

func readUint32(bs []byte, offset int) (uint32, int, error) {
	end := offset + 4
	if end > len(bs) {
		return 0, 0, errOverranBuffer
	}

	return binary.LittleEndian.Uint32(bs[offset:end]), end, nil
}

func readUint16(bs []byte, offset int) (uint16, int, error) {
	end := offset+2
	if end > len(bs) {
		return 0, 0, errOverranBuffer
	}

	return binary.LittleEndian.Uint16(bs[offset:end]), end, nil
}

func readBytes(bs []byte, offset int, n int) ([]byte, int, error) {
	end := offset + n
	if end > len(bs) {
		return nil, 0, errOverranBuffer
	}

	return bs[offset:offset+n], end, nil
}

func readString(bs []byte, offset int, n int) (string, int, error) {
	read, end, err := readBytes(bs, offset, n)
	return string(read), end, err
}

func msdosTimeToGoTime(d uint16, t uint16) time.Time {
	seconds := int((t & 0x1F) * 2)
	minutes := int((t >> 5) & 0x3F)
	hours := int(t >> 11)

	day := int(d & 0x1F)
	month := time.Month((d >> 5) & 0x0F)
	year := int((d >> 9) & 0x7F) + 1980
	return time.Date(year, month, day, hours, minutes, seconds, 0, time.Local)
}


var errNotZip = fmt.Errorf("Not a zip file")

func parseLocalFileHeader(bs []byte, start int) (*localFileHeader, int, error) {
	signature, i, err := readUint32(bs, start)
	if signature != 0x04034b50 {
		return nil, 0, errNotZip
	}
	if err != nil {
		return nil, 0, err
	}

	version, i, err := readUint16(bs, i)
	if err != nil {
		return nil, 0, err
	}

	bitFlag, i, err := readUint16(bs, i)
	if err != nil {
		return nil, 0, err
	}

	compression := noCompression
	compressionRaw, i, err := readUint16(bs, i)
	if err != nil {
		return nil, 0, err
	}
	if compressionRaw == 8 {
		compression = deflateCompression
	}

	lmTime, i, err := readUint16(bs, i)
	if err != nil {
		return nil, 0, err
	}

	lmDate, i, err := readUint16(bs, i)
	if err != nil {
		return nil, 0, err
	}
	lastModified := msdosTimeToGoTime(lmDate, lmTime)

	crc32, i, err := readUint32(bs, i)
	if err != nil {
		return nil, 0, err
	}

	compressedSize, i, err := readUint32(bs, i)
	if err != nil {
		return nil, 0, err
	}

	uncompressedSize, i, err := readUint32(bs, i)
	if err != nil {
		return nil, 0, err
	}

	fileNameLength, i, err := readUint16(bs, i)
	if err != nil {
		return nil, 0, err
	}

	extraFieldLength, i, err := readUint16(bs, i)
	if err != nil {
		return nil, 0, err
	}

	fileName, i, err := readString(bs, i, int(fileNameLength))
	if err != nil {
		return nil, 0, err
	}

	extraField, i, err := readBytes(bs, i, int(extraFieldLength))
	if err != nil {
		return nil, 0, err
	}

	var fileContents string
	if compression == noCompression {
		fileContents, i, err = readString(bs, i, int(uncompressedSize))
		if err != nil {
			return nil, 0, err
		}
	} else {
		end := i + int(compressedSize)
		if end > len(bs) {
			return nil, 0, errOverranBuffer
		}
		flateReader := flate.NewReader(bytes.NewReader(bs[i:end]))

		defer flateReader.Close()
		read, err := ioutil.ReadAll(flateReader)
		if err != nil {
			return nil, 0, err
		}

		fileContents = string(read)

		i = end
	}

	return &localFileHeader{
		signature: signature,
		version: version,
		bitFlag: bitFlag,
		compression: compression,
		lastModified: lastModified,
		crc32: crc32,
		compressedSize: compressedSize,
		uncompressedSize: uncompressedSize,
		fileName: fileName,
		extraField: extraField,
		fileContents: fileContents,
	}, i, nil
}

func main() {
	f, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	end := 0
	for end < len(f) {
		var err error
		var lfh *localFileHeader
		var next int
		lfh, next, err = parseLocalFileHeader(f, end)
		if err == errNotZip && end > 0 {
			break
		}
		if err != nil {
			panic(err)
		}

		end = next

		fmt.Println(lfh.lastModified, lfh.fileName, lfh.fileContents)
	}
}
