package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/sago35/midi"
)

//go:embed "overworld.mid"
var midiData []byte

var (
	baremetal = false
)

func main() {
	if baremetal {
		time.Sleep(3 * time.Second)
	}

	err := run()
	if err != nil {
		log.Fatal(err)
	}

	if baremetal {
		fmt.Printf("done\n")
	}
}

var midiCallback = func(track int, data []byte) {
	//fmt.Printf("callback(%d, %#v)\n", track, data)
}

var ticker = func(tick int) {
	//time.Sleep(time.Duration(tickTimeMicrosecond) * time.Microsecond)
}

var tickTimeMicrosecond uint32 = 5000

func run() error {
	r := bytes.NewReader(midiData)

	m := midi.New(r)
	m.SetCallback(midiCallback)
	m.ParseHeader()

	tickTimeMicrosecond = m.TickTimeMicrosecond()

	for {
		m.Init()
		finished := make([]bool, m.TrackNum())
		for i := 0; ; i++ {
			for no := 0; no < m.TrackNum(); no++ {
				if no != 1 {
					//continue
				}
				//err := m.ParseTrack(no)
				//if err != nil {
				//	return err
				//}
				err := m.TickTrack(no, i)
				if err != nil {
					if errors.Is(err, midi.EndOfTrack) {
						//fmt.Printf("finished %d\n", no)
						finished[no] = true
					} else {
						return err
					}
				}
			}
			end := true
			for _, f := range finished {
				if !f {
					end = false
				}
			}
			if end {
				break
			}
			ticker(i)
		}
	}
	return nil
}
