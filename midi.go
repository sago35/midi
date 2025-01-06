package midi

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type Midi struct {
	r   Reader
	buf [256]byte

	trackPos [16]int64
	trackNum int
}

type Reader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

func New(r Reader) *Midi {
	return &Midi{
		r: r,
	}
}

func (m *Midi) ParseHeader() error {
	var value [4]byte
	binary.Read(m.r, binary.BigEndian, &value)
	//fmt.Printf("%s\n", value)

	var size uint32
	binary.Read(m.r, binary.BigEndian, &size)

	var format uint16
	binary.Read(m.r, binary.BigEndian, &format)

	var trackNum uint16
	binary.Read(m.r, binary.BigEndian, &trackNum)
	m.trackNum = int(trackNum)

	var ticks uint16
	binary.Read(m.r, binary.BigEndian, &ticks)

	cont := true
	idx := 0
	for cont {
		// tracks
		currentPos, err := m.r.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}

		err = binary.Read(m.r, binary.BigEndian, &value)
		if err != nil {
			if errors.Is(err, io.EOF) {
				cont = false
				break
			}
		}
		m.trackPos[idx] = currentPos

		binary.Read(m.r, binary.BigEndian, &size)
		m.r.Seek(int64(size), io.SeekCurrent)
		idx++
	}

	return nil
}

func (m *Midi) TrackNum() int {
	return m.trackNum
}

func (m *Midi) ParseTrack(no int) error {
	//fmt.Printf("-- track %d --\n", no)

	if len(m.trackPos) < no {
		return fmt.Errorf("len(m.trackPos) < no")
	}
	m.r.Seek(m.trackPos[no], io.SeekStart)

	var mtrk [4]byte
	binary.Read(m.r, binary.BigEndian, &mtrk)
	//fmt.Printf("%s\n", mtrk)

	var size uint32
	binary.Read(m.r, binary.BigEndian, &size)
	//fmt.Printf("size   : %04X\n", size)

	remain := size
	var buf [5]byte
	var buf2 [256]byte
	for remain > 0 {
		binary.Read(m.r, binary.BigEndian, buf[:4])

		bufSize := 4
		delta := uint16(buf[0])
		if delta&0x80 == 0x00 {
		} else {
			m.r.Seek(-3, io.SeekCurrent)

			binary.Read(m.r, binary.BigEndian, buf[:4])
			remain -= 1
			bufSize = 5
		}

		sz := buf[3]
		switch buf[1] {
		case 0xFF:
			// meta event
			remain -= 4 + uint32(sz)
			switch buf[2] {
			case 0x03:
				binary.Read(m.r, binary.BigEndian, buf2[:sz])
			case 0x2F:
				// End of track
				if remain != 0 {
					return fmt.Errorf("ParseTrack() : size error")
				}
			case 0x51:
				// Set Tempo
				binary.Read(m.r, binary.BigEndian, buf2[:sz])
			case 0x58:
				// Time Signature
				binary.Read(m.r, binary.BigEndian, buf2[:sz])
			default:
				fmt.Printf("error : unknown buf[2] : %02X\n", buf[2])
				binary.Read(m.r, binary.BigEndian, buf2[:sz])
			}
		default:
			sz = 0
			remain -= 4
			switch buf[1] & 0xF0 {
			case 0xB0:
				// control change
			case 0xC0:
				// program change
				remain += 1
				bufSize -= 1
				m.r.Seek(-1, io.SeekCurrent)
			case 0x90:
			case 0x80:
			default:
				fmt.Printf("error : unknown buf[1] : %02X\n", buf[1])
			}
		}
		if sz == 0 {
			//fmt.Printf("% X\n", buf[:bufSize])
		} else {
			//fmt.Printf("% X % X\n", buf[:bufSize], buf2[:sz])
		}
	}

	return nil
}
