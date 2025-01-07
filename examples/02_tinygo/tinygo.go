//go:build baremetal

package main

import (
	"machine"
	"machine/usb/adc/midi"
	"time"

	"tinygo.org/x/drivers/encoders"
)

const (
	cable = 0
)

func init() {
	baremetal = true

	m := midi.Port()
	midiCallback = func(track int, data []byte) {
		//fmt.Printf("callback(%d, %#v)\n", track, data)
		switch data[1] & 0xF0 {
		case 0xC0:
			channel := (data[1] & 0x0F) + 1
			m.Write(programChange(cable, channel, data[2]))
		case 0x90:
			channel := (data[1] & 0x0F) + 1
			m.NoteOn(cable, channel, midi.Note(data[2]), data[3])
		case 0x80:
			channel := (data[1] & 0x0F) + 1
			m.NoteOff(cable, channel, midi.Note(data[2]), data[3])
		}
	}

	btn := machine.GPIO2
	btn.Configure(machine.PinConfig{Mode: machine.PinInputPullup})

	enc := encoders.NewQuadratureViaInterrupt(
		machine.GPIO3,
		machine.GPIO4,
	)
	enc.Configure(encoders.QuadratureConfig{
		Precision: 4,
	})

	autoPlay := true
	oldValue := 0
	timeout := time.Time{}
	ticker = func(tick int) {
		if autoPlay {
			if !btn.Get() {
				autoPlay = false
				for !btn.Get() {
				}
				timeout = time.Now().Add(2 * time.Second)
			}
			newValue := enc.Position()
			if newValue != oldValue {
				autoPlay = false
				oldValue = newValue
				timeout = time.Now().Add(1 * time.Second)
			}
			time.Sleep(5 * time.Millisecond)
		} else {
			if tick%4 > 0 {
				return
			}
			for {
				if time.Now().After(timeout) {
					autoPlay = true
					return
				}
				newValue := enc.Position()
				if newValue != oldValue {
					oldValue = newValue
					timeout = time.Now().Add(1 * time.Second)
					break
				}
			}
		}
	}
}

var pbuf [4]byte

func programChange(cable, channel uint8, patch uint8) []byte {
	pbuf[0], pbuf[1], pbuf[2], pbuf[3] = ((cable&0xf)<<4)|midi.CINProgramChange, midi.MsgProgramChange|((channel-1)&0xf), patch&0x7f, 0x00
	return pbuf[:4]
}
