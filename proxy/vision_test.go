package proxy_test

import (
	"context"
	"sync"
	"testing"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/uuid"
	. "github.com/xtls/xray-core/proxy"
)

type recordReader struct {
	records [][]byte
}

func (r *recordReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	b := buf.New()
	b.Write(r.records[0])
	r.records = r.records[1:]
	return buf.MultiBuffer{b}, nil
}

type discardWriter struct{}

func (discardWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	buf.ReleaseMulti(mb)
	return nil
}

// tlsRecord returns a 100-byte handshake record of the given type.
func tlsRecord(version, handshakeType byte) []byte {
	b := make([]byte, 100)
	copy(b, []byte{0x16, 0x03, version, 0x00, 95, handshakeType})
	return b
}

// TestVisionTrafficStateRace runs the VisionWriter of one direction and the VisionReader of
// the other one concurrently on a shared TrafficState, as they run in a real connection.
// Both call XtlsFilterTls, so the filter fields must be safe for concurrent use; run with -race.
func TestVisionTrafficStateRace(t *testing.T) {
	id := uuid.New()

	// TLS 1.3 ServerHello with TLS_AES_128_GCM_SHA256
	serverHello := tlsRecord(0x03, 0x02)
	serverHello[44], serverHello[45] = 0x13, 0x01
	copy(serverHello[60:], Tls13SupportedVersions)

	for range 200 {
		state := NewTrafficState(id.Bytes())
		reader := NewVisionReader(&recordReader{records: [][]byte{serverHello, make([]byte, 100), make([]byte, 100)}}, state, false, context.Background(), nil, nil, nil, nil)
		writer := NewVisionWriter(discardWriter{}, state, true, context.Background(), nil, nil, nil)

		var wg sync.WaitGroup
		wg.Go(func() {
			for range 3 {
				mb, err := reader.ReadMultiBuffer()
				if err != nil {
					t.Error(err)
				}
				buf.ReleaseMulti(mb)
			}
		})
		wg.Go(func() {
			for range 4 {
				b := buf.New()
				b.Write(tlsRecord(0x01, 0x01))
				if err := writer.WriteMultiBuffer(buf.MultiBuffer{b}); err != nil {
					t.Error(err)
				}
			}
		})
		wg.Wait()
	}
}
