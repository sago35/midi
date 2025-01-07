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
	trackTim [16]uint32
	trackSiz [16]uint32
	trackNum int

	callback func(track int, data []byte)

	tempo uint32
	ticks uint16

	lastCommand uint8
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

func (m *Midi) TickTimeMicrosecond() uint32 {
	return m.tempo / uint32(m.ticks)
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
	m.ticks = ticks

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

	m.TickTrack(0, 0)

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

	var buf [3]byte
	var buf2 [256]byte
	var buf3 [4]byte
	cont := true
	for cont {
		binary.Read(m.r, binary.BigEndian, buf3[:4])

		bufSize := int64(3)
		delta := uint32(0)
		deltaSize := 0
		for i := 0; i < 4; i++ {
			delta = (delta << 7) + uint32(buf3[i]&0x7F)
			deltaSize = i + 1
			if buf3[i]&0x80 == 0 {
				break
			}
		}
		if deltaSize != 4 {
			m.r.Seek(-4+int64(deltaSize), io.SeekCurrent)
		}
		binary.Read(m.r, binary.BigEndian, buf[:bufSize])

		//fmt.Printf("tim+delta %d : tick %d : delta %d\n", m.trackTim[no]+delta, uint16(tick), delta)
		if m.trackTim[no]+delta > uint32(tick) {
			m.r.Seek(-1*(int64(deltaSize)+bufSize), io.SeekCurrent)
			break
		}

		sz := buf[2]
		switch buf[0] {
		case 0xFF:
			// meta event
			switch buf[1] {
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
				m.tempo = uint32(buf2[0])<<16 | uint32(buf2[1])<<8 | uint32(buf2[2])
			case 0x58:
				// Time Signature
				binary.Read(m.r, binary.BigEndian, buf2[:sz])
			case 0x59:
				// Key Signature
				binary.Read(m.r, binary.BigEndian, buf2[:sz])
			default:
				//fmt.Printf("error : unknown buf[1] : %02X\n", buf[1])
				binary.Read(m.r, binary.BigEndian, buf2[:sz])
			}
		default:
			sz = 0
			switch buf[0] & 0xF0 {
			case 0x80:
				// Note Off
				m.lastCommand = buf[0]
			case 0x90:
				// Note On
				m.lastCommand = buf[0]
			case 0xA0:
				// Polyphonic Key Pressure
				m.lastCommand = buf[0]
			case 0xB0:
				// Control Change
				m.lastCommand = buf[0]
			case 0xC0:
				// Program Change
				bufSize = 2
				m.r.Seek(-1, io.SeekCurrent)
				m.lastCommand = buf[0]
			case 0xD0:
				// Channel Pressure
				bufSize = 2
				m.r.Seek(-1, io.SeekCurrent)
				m.lastCommand = buf[0]
			case 0xE0:
				// Pitch Bend Change
				m.lastCommand = buf[0]
			default:
				//fmt.Printf("error : unknown buf[0]=%02X : %d % X\n", buf[0], no, buf)
				switch m.lastCommand {
				case 0xC0, 0xD0:
					bufSize = 1
				default:
					bufSize = 2
				}
				m.r.Seek(-1, io.SeekCurrent)
				buf[0], buf[1], buf[2] = m.lastCommand, buf[0], buf[1]
			}
			m.callback(no, buf[:])
		}
		if sz == 0 {
			//fmt.Printf("%d - %04X / %04X - % X - % X\n", no, m.trackOfs[no]+int64(m.trackSiz[no]), m.trackPtr[no], buf3[:deltaSize], buf[:])
		} else {
			//fmt.Printf("%d - %04X / %04X - % X - % X - % X\n", no, m.trackOfs[no]+int64(m.trackSiz[no]), m.trackPtr[no], buf3[:deltaSize], buf[:bufSize], buf2[:sz])
		}
		m.trackPtr[no] += int64(deltaSize) + bufSize + int64(sz)
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
