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

	trackOfs [16]int64
	trackPtr [16]int64
	trackTim [16]uint16
	trackSiz [16]uint32
	trackNum int

	callback func(track int, data []byte)
}

type Reader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

func New(r Reader) *Midi {
	return &Midi{
		r:        r,
		callback: func(track int, data []byte) {},
	}
}

func (m *Midi) Init() {
	for i := range m.trackPtr {
		m.trackPtr[i] = 0
		m.trackTim[i] = 0
	}
}

func (m *Midi) SetCallback(fn func(track int, data []byte)) {
	m.callback = fn
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
		m.trackOfs[idx] = currentPos

		binary.Read(m.r, binary.BigEndian, &size)
		m.r.Seek(int64(size), io.SeekCurrent)
		idx++
	}

	return nil
}

func (m *Midi) TrackNum() int {
	return m.trackNum
}

func (m *Midi) TickTrack(no, tick int) error {
	if len(m.trackOfs) < no {
		return fmt.Errorf("len(m.trackOfs) < no")
	}
	if m.trackOfs[no]+int64(m.trackSiz[no]) < m.trackPtr[no] {
		return nil
	}

	//fmt.Printf("-- tick %d --\n", tick)
	//fmt.Printf("ofs %04X\n", m.trackOfs[no])
	//fmt.Printf("ptr %04X\n", m.trackPtr[no])
	//fmt.Printf("tim %04X\n", m.trackTim[no])

	if m.trackPtr[no] == 0 {
		m.trackPtr[no] = m.trackOfs[no]
		m.r.Seek(m.trackPtr[no], io.SeekStart)
		var mtrk [4]byte
		binary.Read(m.r, binary.BigEndian, &mtrk)

		var size uint32
		binary.Read(m.r, binary.BigEndian, &size)
		m.trackSiz[no] = size

		m.trackPtr[no] += 8
	}
	m.r.Seek(m.trackPtr[no], io.SeekStart)

	var buf [5]byte
	var buf2 [256]byte
	cont := true
	for cont {
		binary.Read(m.r, binary.BigEndian, buf[:4])

		bufSize := int64(4)
		delta := uint16(buf[0])
		if delta&0x80 == 0x00 {
		} else {
			m.r.Seek(-3, io.SeekCurrent)

			binary.Read(m.r, binary.BigEndian, buf[:4])
			bufSize = 5

			delta = ((delta & 0x7F) << 7) | uint16(buf[0])
		}

		//fmt.Printf("tim+delta %d : tick %d : delta %d\n", m.trackTim[no]+delta, uint16(tick), delta)
		if m.trackTim[no]+delta > uint16(tick) {
			m.r.Seek(-1*bufSize, io.SeekCurrent)
			break
		}

		sz := buf[3]
		switch buf[1] {
		case 0xFF:
			// meta event
			switch buf[2] {
			case 0x03:
				binary.Read(m.r, binary.BigEndian, buf2[:sz])
			case 0x2F:
				// End of track
				//return fmt.Errorf("end of track")
				//return nil
				cont = false
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
			switch buf[1] & 0xF0 {
			case 0xB0:
				// control change
			case 0xC0:
				// program change
				bufSize -= 1
				m.r.Seek(-1, io.SeekCurrent)
			case 0x90:
				// note on
			case 0x80:
				// note off
			default:
				fmt.Printf("error : unknown buf[1] : %02X\n", buf[1])
			}
			m.callback(no, buf[:bufSize])
		}
		if sz == 0 {
			//fmt.Printf("%d % X\n", no, buf[:bufSize])
		} else {
			//fmt.Printf("%d % X % X\n", no, buf[:bufSize], buf2[:sz])
		}
		m.trackPtr[no] += bufSize + int64(sz)
		m.trackTim[no] += delta
	}

	if cont == false {
		return EndOfTrack
	}

	return nil
}

var (
	EndOfTrack = errors.New("end of track")
)

func (m *Midi) ParseTrack(no int) error {
	//fmt.Printf("-- track %d --\n", no)

	if len(m.trackOfs) < no {
		return fmt.Errorf("len(m.trackOfs) < no")
	}
	m.r.Seek(m.trackOfs[no], io.SeekStart)

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
