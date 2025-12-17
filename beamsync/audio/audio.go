package audio

import (
	"io"
	"log"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

type AudioEngine struct {
	buffers     map[string]*beep.Buffer
	initialized bool
	sampleRate  beep.SampleRate
}

func NewAudioEngine() *AudioEngine {
	return &AudioEngine{
		buffers:    make(map[string]*beep.Buffer),
		sampleRate: beep.SampleRate(44100),
	}
}

func (ae *AudioEngine) Init() error {
	// 100ms buffer size for low latency but safe from shuttering
	// For "zero-latency" feel, we might want lower, e.g. time.Second/30 or /60
	err := speaker.Init(ae.sampleRate, ae.sampleRate.N(time.Second/30))
	if err != nil {
		return err
	}
	ae.initialized = true
	return nil
}

func (ae *AudioEngine) LoadSound(name string, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	return ae.LoadSoundFromStream(name, f)
}

func (ae *AudioEngine) LoadSoundFromStream(name string, rc io.ReadCloser) error {
	defer rc.Close()

	streamer, format, err := wav.Decode(rc)
	if err != nil {
		return err
	}

	// Resample if necessary to match speaker
	var s beep.Streamer = streamer
	if format.SampleRate != ae.sampleRate {
		s = beep.Resample(4, format.SampleRate, ae.sampleRate, streamer)
	}

	buffer := beep.NewBuffer(beep.Format{
		SampleRate:  ae.sampleRate,
		NumChannels: 2,
		Precision:   2,
	})
	buffer.Append(s)
	streamer.Close()

	ae.buffers[name] = buffer
	return nil
}

func (ae *AudioEngine) Play(name string) {
	if !ae.initialized {
		return
	}
	buffer, ok := ae.buffers[name]
	if !ok {
		log.Printf("AudioEngine: Sound '%s' not found", name)
		return
	}

	s := buffer.Streamer(0, buffer.Len())
	speaker.Play(s)
}
