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
	fmt.Printf("%s\n", value)

	var size uint32
	binary.Read(m.r, binary.BigEndian, &size)
	fmt.Printf("size   : %04X\n", size)

	var format uint16
	binary.Read(m.r, binary.BigEndian, &format)
	fmt.Printf("format : %04X\n", format)

	var trackNum uint16
	binary.Read(m.r, binary.BigEndian, &trackNum)
	fmt.Printf("tracks : %04X\n", trackNum)

	var ticks uint16
	binary.Read(m.r, binary.BigEndian, &ticks)
	fmt.Printf("ticks  : %04X\n", ticks)

	fmt.Printf("\n")
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
	num := 0
	for i, ofs := range m.trackPos {
		if ofs == 0 {
			break
		}
		num = i
	}
	return num + 1
}

func (m *Midi) ParseTrack(no int) error {
	fmt.Printf("-- track %d --\n", no)

	if len(m.trackPos) < no {
		return fmt.Errorf("len(m.trackPos) < no")
	}
	m.r.Seek(m.trackPos[no], io.SeekStart)

	var mtrk [4]byte
	binary.Read(m.r, binary.BigEndian, &mtrk)
	fmt.Printf("%s\n", mtrk)

	var size uint32
	binary.Read(m.r, binary.BigEndian, &size)
	fmt.Printf("size   : %04X\n", size)

	remain := size
	for remain > 0 {
		var buf [256]byte
		binary.Read(m.r, binary.BigEndian, buf[:4])

		delta := uint16(buf[0])
		if delta&0x80 == 0x00 {
			fmt.Printf("buf  : % X\n", buf[:4])
		} else {
			m.r.Seek(-3, io.SeekCurrent)

			binary.Read(m.r, binary.BigEndian, buf[:4])
			fmt.Printf("buf  : %02X % X\n", delta, buf[:4])
		}

		switch buf[1] {
		case 0xFF:
			// meta event
			remain -= 4 + uint32(buf[3])
			switch buf[2] {
			case 0x03:
				binary.Read(m.r, binary.BigEndian, buf[:buf[3]])
			case 0x2F:
				// End of track
				remain = 0
			case 0x51:
				// Set Tempo
				binary.Read(m.r, binary.BigEndian, buf[:buf[3]])
			case 0x58:
				// Time Signature
				binary.Read(m.r, binary.BigEndian, buf[:buf[3]])
			default:
				fmt.Printf("error : unknown buf[2] : %02X\n", buf[2])
				binary.Read(m.r, binary.BigEndian, buf[:buf[3]])
			}
		default:
			switch buf[1] & 0xF0 {
			case 0xB0:
				// control change
			case 0xC0:
				// program change
				m.r.Seek(-1, io.SeekCurrent)
			case 0x90:
			case 0x80:
			default:
				fmt.Printf("error : unknown buf[1] : %02X\n", buf[1])
			}
		}
	}

	return nil
}
