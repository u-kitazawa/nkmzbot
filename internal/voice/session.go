package voice

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"layeh.com/gopus"
)

type Session struct {
	GuildID string
	Conn    *discordgo.VoiceConnection
	Quit    chan bool
	Streams map[uint32]*UserStream
}

type UserStream struct {
	File    *os.File
	Encoder *wav.Encoder
	Decoder *gopus.Decoder
	Path    string
}

func NewSession(guildID string, conn *discordgo.VoiceConnection) *Session {
	return &Session{
		GuildID: guildID,
		Conn:    conn,
		Quit:    make(chan bool),
		Streams: make(map[uint32]*UserStream),
	}
}

func (s *Session) StartRecording() {
	go s.listen()
}

func (s *Session) StopRecording() []string {
	close(s.Quit)
	var files []string
	for _, stream := range s.Streams {
		stream.Encoder.Close()
		stream.File.Close()
		files = append(files, stream.Path)
	}
	return files
}

func (s *Session) listen() {
	for {
		select {
		case <-s.Quit:
			return
		case packet, ok := <-s.Conn.OpusRecv:
			if !ok {
				return
			}
			s.handlePacket(packet)
		}
	}
}

func (s *Session) handlePacket(p *discordgo.Packet) {
	stream, ok := s.Streams[p.SSRC]
	if !ok {
		var err error
		stream, err = s.createStream(p.SSRC)
		if err != nil {
			fmt.Printf("Error creating stream: %v\n", err)
			return
		}
		s.Streams[p.SSRC] = stream
	}

	// Decode Opus to PCM
	// Discord sends stereo 48kHz audio (usually). Frame size 960 (20ms)
	pcm, err := stream.Decoder.Decode(p.Opus, 960, false)
	if err != nil {
		fmt.Printf("Error decoding opus: %v\n", err)
		return
	}

	// Create IntBuffer for WAV encoder
	// PCM data is interleaved stereo (L R L R ...)
	buf := &audio.IntBuffer{
		Format: &audio.Format{
			SampleRate:  48000,
			NumChannels: 2,
		},
		Data:           make([]int, len(pcm)),
		SourceBitDepth: 16,
	}

	for i, v := range pcm {
		buf.Data[i] = int(v)
	}

	if err := stream.Encoder.Write(buf); err != nil {
		fmt.Printf("Error writing to WAV: %v\n", err)
	}
}

func (s *Session) createStream(ssrc uint32) (*UserStream, error) {
	// Ensure directory exists
	dir := filepath.Join("recordings", s.GuildID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("%d_%d.wav", ssrc, time.Now().Unix())
	path := filepath.Join(dir, filename)

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	// Setup WAV encoder: 48kHz, 16bit, 2ch, PCM (1)
	enc := wav.NewEncoder(f, 48000, 16, 2, 1)

	// Setup Opus decoder: 48kHz, 2ch
	dec, err := gopus.NewDecoder(48000, 2)
	if err != nil {
		f.Close()
		return nil, err
	}

	return &UserStream{
		File:    f,
		Encoder: enc,
		Decoder: dec,
		Path:    path,
	}, nil
}
