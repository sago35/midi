package main

import (
	"bytes"
	_ "embed"
	"log"
	"time"

	"github.com/sago35/midi"
)

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
}

//go:embed "overworld.mid"
var midiData []byte

func run() error {
	r := bytes.NewReader(midiData)
	m := midi.New(r)

	m.ParseHeader()

	for no := 0; no < m.TrackNum(); no++ {
		if no != 1 {
			//continue
		}
		m.ParseTrack(no)
	}
	return nil
}
