package fastgelf

import (
	"log"
	"net"
	"os"
	"time"

	"github.com/francoispqt/gojay"
	"golang.org/x/net/ipv4"
)

const (
	defaultFlushInterval = time.Second
	maxBatchSize         = 1024
)

var Logger = log.New(os.Stderr, "", log.LstdFlags)

type unsafeWriter struct {
	data [1][]byte
	enc  *gojay.Encoder
}

func (w *unsafeWriter) Write(data []byte) (int, error) {
	w.data[0] = data
	return len(data), nil
}

type UDPWriter struct {
	FlushInterval time.Duration
	ch            chan *unsafeWriter
	conn          *ipv4.PacketConn
	addr          *net.UDPAddr
	done          chan struct{}
}

func NewUDPWriter(addr string) (*UDPWriter, error) {
	w := &UDPWriter{
		done:          make(chan struct{}),
		ch:            make(chan *unsafeWriter, maxBatchSize),
		FlushInterval: defaultFlushInterval,
	}
	var err error
	w.addr, err = net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	w.conn = ipv4.NewPacketConn(conn)
	go w.flush()
	return w, nil
}

func (w *UDPWriter) Close() error {
	close(w.ch)
	<-w.done
	return w.conn.Close()
}

func (w *UDPWriter) enqueue(uw *unsafeWriter) {
	w.ch <- uw
}

func (w *UDPWriter) flush() {
	defer close(w.done)
	t := time.NewTimer(w.FlushInterval)
	var flush, done bool
	var ipMsgs [maxBatchSize]ipv4.Message
	var msgs [maxBatchSize]*unsafeWriter
	var pos int
	for {
		select {
		case msg := <-w.ch:
			if msg == nil {
				flush = true
				done = true
				break
			}
			if pos == maxBatchSize {
				pos--
			}
			msgs[pos] = msg
			ipMsgs[pos] = ipv4.Message{
				Buffers: msg.data[:],
				Addr:    w.addr,
			}
			pos++
			flush = pos == maxBatchSize
		case <-t.C:
			flush = true
			t.Reset(w.FlushInterval)
		}
		if flush && pos > 0 {
			for skip := 0; skip < pos; {
				written, err := w.conn.WriteBatch(ipMsgs[skip:pos], 0)
				if err != nil {
					Logger.Printf("error writing message batch: %+v", err)
					skip++
				} else {
					skip += written
				}
			}
			for i := 0; i < pos; i++ {
				msgs[i].enc.Release()
			}
			pos = 0
		}
		if done {
			break
		}
	}
}

func (w *UDPWriter) WriteMessage(msg *Message) error {
	uw := &unsafeWriter{}
	enc := gojay.BorrowEncoder(uw)
	uw.enc = enc
	err := enc.EncodeObject(msg)
	if err != nil {
		return err
	}
	w.enqueue(uw)
	return nil
}
