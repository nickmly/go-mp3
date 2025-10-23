package player

import (
	"errors"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
	"github.com/gopxl/beep/wav"
	"github.com/rivo/tview"
)

const (
	PlayIcon  string = "⏵"
	PauseIcon string = "⏸"
	StopIcon  string = "⏹"
	NextIcon  string = "⏭"
	PrevIcon  string = "⏮"
)

var decoders = map[string]func(io.ReadCloser) (s beep.StreamSeekCloser, format beep.Format, err error){
	".mp3": mp3.Decode,
	".wav": func(rc io.ReadCloser) (s beep.StreamSeekCloser, format beep.Format, err error) {
		return wav.Decode(rc)
	},
	".ogg": vorbis.Decode,
}

type PlayerControls struct {
	cursor  int
	buttons []tview.Primitive
}

func (pc *PlayerControls) GoRight() int {
	pc.cursor = (pc.cursor + 1) % len(pc.buttons)
	return pc.cursor
}

func (pc *PlayerControls) GoLeft() int {
	pc.cursor = (pc.cursor - 1 + len(pc.buttons)) % len(pc.buttons)
	return pc.cursor
}

type PlayerState struct {
	playing            bool
	currentSongIndex   int
	controls           PlayerControls
	songList           []string
	streamer           beep.StreamSeekCloser
	ctrl               *beep.Ctrl
	volume             *effects.Volume
	currentVolume      float64
	sampleRate         beep.SampleRate
	OnInput            func(event *tcell.EventKey) *tcell.EventKey
	OnSongChanged      func()
	cancelChan         chan struct{}
	speakerInitialized bool
}

func (ps *PlayerState) finishCurrentSong() {
	speaker.Lock()
	defer speaker.Unlock()
	if ps.cancelChan != nil {
		close(ps.cancelChan)
		ps.cancelChan = nil
	}
	if ps.streamer == nil {
		return
	}
	ps.streamer.Close()
	ps.streamer = nil
	if ps.ctrl != nil {
		ps.ctrl.Paused = true
	}
}

func (ps *PlayerState) createStreamerFromFile() error {
	ps.cancelChan = make(chan struct{})
	filePath := ps.songList[ps.currentSongIndex]
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	extension := filepath.Ext(filePath)
	var streamer beep.StreamSeekCloser
	var format beep.Format
	decoder := decoders[extension]
	if decoder == nil {
		log.Fatal("unsupported extension " + extension)
	}
	streamer, format, err = decoder(f)
	if err != nil {
		log.Fatal(err)
	}
	var ctrl *beep.Ctrl
	ps.streamer = streamer

	if !ps.speakerInitialized {
		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		ps.speakerInitialized = true
		ps.sampleRate = format.SampleRate
	}

	var playStreamer beep.Streamer = streamer
	if format.SampleRate != ps.sampleRate {
		playStreamer = beep.Resample(3, format.SampleRate, ps.sampleRate, streamer)
	}

	ctrl = &beep.Ctrl{Streamer: playStreamer, Paused: !ps.playing}
	ps.ctrl = ctrl
	// effects.Volume uses a logarithmic Volume value. Convert multiplier -> log2(multiplier).
	vol := ps.currentVolume
	v := 0.0
	if vol > 0 {
		v = math.Log2(vol)
	}
	ps.volume = &effects.Volume{
		Base:     2,
		Volume:   v,
		Streamer: ctrl,
		Silent:   vol == 0,
	}
	go func(cancel <-chan struct{}) {
		finished := make(chan bool)
		speaker.Play(beep.Seq(ps.volume, beep.Callback(func() {
			finished <- true
		})))

		select {
		case <-finished:
			ps.NextSong()
			if ps.OnSongChanged != nil {
				ps.OnSongChanged()
			}
		case <-cancel:
			// user manually interrupted
			return
		}
	}(ps.cancelChan)

	return nil
}

func NewPlayerState() *PlayerState {
	return &PlayerState{
		playing:          false,
		currentSongIndex: 0,
		songList:         []string{},
		// default to 100% (1.0 multiplier)
		currentVolume: 1.0,
	}
}

func (ps *PlayerState) AddPlayerControls(startCursor int, buttons []tview.Primitive) PlayerControls {
	pc := PlayerControls{
		cursor:  startCursor,
		buttons: buttons,
	}
	ps.controls = pc
	return pc
}

func (ps *PlayerState) ReadSongListFromDir(dirPath string) ([]string, error) {
	dir, err := os.ReadDir(dirPath)
	if err != nil {
		return []string{}, err
	}
	songList := []string{}
	for _, entry := range dir {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		fileExtension := filepath.Ext(fileName)
		extensionSupported := decoders[fileExtension] != nil
		if !extensionSupported {
			continue
		}
		songList = append(songList, filepath.Join(dirPath, fileName))
	}
	if len(songList) == 0 {
		return []string{}, errors.New("song list is empty")
	}
	ps.songList = songList
	ps.updateSongList(songList)
	return songList, nil
}

func (ps *PlayerState) PlaySong() bool {
	if ps.ctrl == nil {
		return false
	}
	speaker.Lock()
	ps.ctrl.Paused = !ps.ctrl.Paused
	speaker.Unlock()
	return true
}

func (ps *PlayerState) NextSong() bool {
	if len(ps.songList) == 0 {
		return false
	}
	ps.currentSongIndex = (ps.currentSongIndex + 1) % len(ps.songList)
	ps.finishCurrentSong()
	ps.createStreamerFromFile()
	return true
}

func (ps *PlayerState) PrevSong() bool {
	if len(ps.songList) == 0 {
		return false
	}
	ps.currentSongIndex = (ps.currentSongIndex - 1 + len(ps.songList)) % len(ps.songList)
	ps.finishCurrentSong()
	ps.createStreamerFromFile()
	return true
}

func (ps *PlayerState) updateSongList(songList []string) {
	if len(ps.songList) == 0 {
		return
	}
	ps.songList = songList
	ps.currentSongIndex = 0
	ps.finishCurrentSong()
	ps.createStreamerFromFile()
}

func (ps *PlayerState) Shutdown() {
	if ps.streamer != nil {
		ps.streamer.Close()
	}
}

func (ps *PlayerState) TogglePlaying() {
	ps.playing = !ps.playing
}

func (ps *PlayerState) IsPlaying() bool {
	return ps.playing
}

func (ps *PlayerState) CurrentSongList() []string {
	return ps.songList
}

func (ps *PlayerState) CurrentSongIndex() int {
	return ps.currentSongIndex
}

func (ps *PlayerState) IncreaseVolume() float64 {
	ps.currentVolume = math.Min(2.0, ps.currentVolume*1.1)
	if ps.volume != nil {
		if ps.currentVolume == 0 {
			ps.volume.Silent = true
			ps.volume.Volume = 0
		} else {
			ps.volume.Silent = false
			ps.volume.Volume = math.Log2(ps.currentVolume)
		}
	}
	return ps.currentVolume * 100
}

func (ps *PlayerState) DecreaseVolume() float64 {
	ps.currentVolume = math.Max(0.0, ps.currentVolume*0.9)
	if ps.volume != nil {
		if ps.currentVolume == 0 {
			ps.volume.Silent = true
			ps.volume.Volume = 0
		} else {
			ps.volume.Silent = false
			ps.volume.Volume = math.Log2(ps.currentVolume)
		}
	}
	return ps.currentVolume * 100
}
