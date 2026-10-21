// game/tape.go
package game

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/txny34/futenis/sim"
)

// Tape v2: a whole match as start setup + config + the human slots' inputs per
// step (only the slots that changed; equal consecutive steps are merged). The
// server records online matches with it and the client replays them through
// the same deterministic Session (client/src/replay/tape-format.ts mirrors it).
const (
	TapeMagic       = 0x32505446 // "FTP2"
	TapeVersion     = 2
	TapeHeaderLen   = 16 + CfgBytes
	tapeMaxRunTicks = 0xFFFF
	tapeInputBytes  = 6 * 8
)

var ErrBadTape = errors.New("game: malformed tape")

// TapeSetup is the configure() call a tape starts from.
type TapeSetup struct {
	Mode, Difficulty, PointsToWin, Mask, Seat, FirstServer int
}

// TapeWriter appends steps; Bytes returns nil once the tape outgrew maxTicks.
type TapeWriter struct {
	buf      []byte
	prev     [sim.MaxPlayers]sim.PlayerInput
	prevMask uint8
	last     int // offset of the last record, -1 = none
	records  uint32
	ticks    int
	maxTicks int
}

// NewTapeWriter starts a tape from setup and cfg (CfgBytes from WriteConfig).
func NewTapeWriter(s TapeSetup, cfg []byte, maxTicks int) *TapeWriter {
	w := &TapeWriter{buf: make([]byte, TapeHeaderLen, TapeHeaderLen+64*1024), prevMask: uint8(s.Mask), last: -1, maxTicks: maxTicks}
	b := w.buf
	binary.LittleEndian.PutUint32(b[0:], TapeMagic)
	b[4], b[5], b[6], b[7] = TapeVersion, byte(s.Mode), byte(s.Difficulty), byte(s.Mask)
	binary.LittleEndian.PutUint16(b[8:], uint16(s.PointsToWin))
	b[10], b[11] = byte(s.Seat), byte(s.FirstServer)
	copy(b[16:TapeHeaderLen], cfg)
	return w
}

// Frame records n ticks run with in under mask.
func (w *TapeWriter) Frame(in *[sim.MaxPlayers]sim.PlayerInput, n, mask int) {
	if w.ticks > w.maxTicks {
		return
	}
	w.ticks += n
	m := uint8(mask)
	var changed uint8
	for i := 0; i < sim.MaxPlayers; i++ {
		if m&(1<<i) != 0 && in[i] != w.prev[i] {
			changed |= 1 << i
		}
	}
	if w.last >= 0 && changed == 0 && m == w.prevMask {
		t := int(binary.LittleEndian.Uint16(w.buf[w.last:]))
		if t+n <= tapeMaxRunTicks {
			binary.LittleEndian.PutUint16(w.buf[w.last:], uint16(t+n))
			return
		}
	}
	w.appendRecord(in, n, m, changed)
}

func (w *TapeWriter) appendRecord(in *[sim.MaxPlayers]sim.PlayerInput, n int, m, changed uint8) {
	w.last = len(w.buf)
	var head [4]byte
	binary.LittleEndian.PutUint16(head[:], uint16(n))
	head[2], head[3] = m, changed
	w.buf = append(w.buf, head[:]...)
	for i := 0; i < sim.MaxPlayers; i++ {
		if changed&(1<<i) == 0 {
			continue
		}
		w.buf = appendInput(w.buf, in[i])
		w.prev[i] = in[i]
	}
	w.prevMask = m
	w.records++
}

func appendInput(b []byte, p sim.PlayerInput) []byte {
	for _, f := range [6]float64{p.MoveX, p.MoveZ, p.TargetX, p.TargetZ, float64(p.Buttons), float64(p.Spin)} {
		b = binary.LittleEndian.AppendUint64(b, math.Float64bits(f))
	}
	return b
}

// Bytes finalizes the record count and returns the tape (nil when too long).
func (w *TapeWriter) Bytes() []byte {
	if w.ticks > w.maxTicks {
		return nil
	}
	binary.LittleEndian.PutUint32(w.buf[12:], w.records)
	return w.buf
}

// TapeStep is one decoded record: every slot's carried-forward input.
type TapeStep struct {
	Ticks int
	Mask  int
	In    [sim.MaxPlayers]sim.PlayerInput
}

// DecodeTape parses a tape (tests and tools; the browser decodes in TS).
func DecodeTape(b []byte) (TapeSetup, []byte, []TapeStep, error) {
	if len(b) < TapeHeaderLen || binary.LittleEndian.Uint32(b) != TapeMagic || b[4] != TapeVersion {
		return TapeSetup{}, nil, nil, ErrBadTape
	}
	s := TapeSetup{Mode: int(b[5]), Difficulty: int(b[6]), Mask: int(b[7]), PointsToWin: int(binary.LittleEndian.Uint16(b[8:])), Seat: int(b[10]), FirstServer: int(b[11])}
	n := int(binary.LittleEndian.Uint32(b[12:]))
	steps := make([]TapeStep, 0, n)
	var cur [sim.MaxPlayers]sim.PlayerInput
	off := TapeHeaderLen
	for r := 0; r < n; r++ {
		st, next, err := decodeRecord(b, off, &cur)
		if err != nil {
			return s, nil, nil, err
		}
		steps = append(steps, st)
		off = next
	}
	return s, b[16:TapeHeaderLen], steps, nil
}

func decodeRecord(b []byte, off int, cur *[sim.MaxPlayers]sim.PlayerInput) (TapeStep, int, error) {
	if off+4 > len(b) {
		return TapeStep{}, 0, ErrBadTape
	}
	st := TapeStep{Ticks: int(binary.LittleEndian.Uint16(b[off:])), Mask: int(b[off+2])}
	changed := b[off+3]
	off += 4
	for i := 0; i < sim.MaxPlayers; i++ {
		if changed&(1<<i) == 0 {
			continue
		}
		if off+tapeInputBytes > len(b) {
			return TapeStep{}, 0, ErrBadTape
		}
		f := func(k int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(b[off+k*8:])) }
		cur[i] = sim.PlayerInput{MoveX: f(0), MoveZ: f(1), TargetX: f(2), TargetZ: f(3), Buttons: uint8(f(4)), Spin: int8(f(5))}
		off += tapeInputBytes
	}
	st.In = *cur
	return st, off, nil
}
