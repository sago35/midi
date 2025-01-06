package main

import (
	"bytes"
	_ "embed"
	"fmt"
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

	if baremetal {
		fmt.Printf("done\n")
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
		err := m.ParseTrack(no)
		if err != nil {
			return err
		}
	}
	return nil
}
