package match

import (
	"bytes"
	"compress/gzip"

	"github.com/txny34/futenis/ai"
	"github.com/txny34/futenis/cmd/server/internal/wire"
	"github.com/txny34/futenis/game"
	"github.com/txny34/futenis/sim"
)

// MaxTapeTicks caps a recorded match (20 min): longer ones are not sent.
const MaxTapeTicks = 20 * 60 * sim.TickRate

// newRoomTape starts recording the match s was just configured for.
func newRoomTape(s *game.Session, mode sim.Mode, mask int) *game.TapeWriter {
	cfg, scratch := make([]byte, game.CfgBytes), make([]float64, sim.CfgLen)
	s.WriteConfig(cfg, scratch)
	setup := game.TapeSetup{Mode: int(mode), Difficulty: int(ai.Normal), PointsToWin: roomPointsToWin, Mask: mask, FirstServer: s.FirstServer()}
	return game.NewTapeWriter(setup, cfg, MaxTapeTicks)
}

// tapeMessages gzips the tape into TAPE chunks; nil when there is no tape.
func tapeMessages(tape []byte) [][]byte {
	if tape == nil {
		return nil
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(tape); err != nil {
		return nil
	}
	if err := zw.Close(); err != nil {
		return nil
	}
	return wire.EncodeTapeChunks(buf.Bytes())
}

// sendTape delivers the chunks to every seated peer, off the sim loop.
func sendTape(peers [sim.MaxPlayers]Peer, msgs [][]byte) {
	if msgs == nil {
		return
	}
	go func() {
		for _, p := range peers {
			if p == nil {
				continue
			}
			for _, m := range msgs {
				if p.Send(m) != nil {
					break // gone or too slow: the replay is a bonus, never block
				}
			}
		}
	}()
}
