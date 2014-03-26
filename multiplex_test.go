package multiplex

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"testing"

	"github.com/stretchrcom/testify/assert"
)

type rwc struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (r *rwc) Read(b []byte) (int, error) {
	return r.r.Read(b)
}

func (r *rwc) Write(b []byte) (int, error) {
	return r.w.Write(b)
}

func (r *rwc) Close() error {
	r.r.Close()
	return r.w.Close()
}

func newServerAndClient() (s *MultiplexedStream, c *MultiplexedStream) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	sconn := &rwc{r: sr, w: sw}
	cconn := &rwc{r: cr, w: cw}

	s = MultiplexedServer(sconn)
	c = MultiplexedClient(cconn)
	return
}

func writepacket(w io.Writer, msg string, id uint32) error {
	if _, err := w.Write([]byte(msg)[:8]); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, id)
}

func readpacket(r io.Reader) (msg string, id uint32, err error) {
	buf := make([]byte, 8)
	if _, err = io.ReadFull(r, buf); err != nil {
		return
	}
	msg = string(buf)
	err = binary.Read(r, binary.LittleEndian, &id)
	return
}

func BenchmarkClientServer(b *testing.B) {
	sm, cm := newServerAndClient()
	wg := &sync.WaitGroup{}

	wg.Add(1)

	// Server
	go func() {
		buf := make([]byte, 1024)
		defer wg.Done()
		ch, err := sm.Accept()
		assert.NoError(b, err)
		if err != nil {
			return
		}
		defer ch.Close()
		for i := 0; i < b.N; i++ {
			_, err := ch.Write(buf)
			assert.NoError(b, err)
			if err != nil {
				return
			}
		}
	}()

	ch, err := cm.Dial()
	assert.NoError(b, err)

	buf := make([]byte, 1024)
	for i := 0; i < b.N; i++ {
		n, err := ch.Read(buf)
		assert.NoError(b, err)
		if err != nil {
			b.FailNow()
		}
		assert.Equal(b, 1024, n)
	}

	sm.Close()
	cm.Close()
	wg.Wait()
}

func TestChannelClientClose(t *testing.T) {
	sm, cm := newServerAndClient()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := sm.Accept()
		assert.NoError(t, err)
		b := make([]byte, 4)
		_, err = c.Read(b)
		assert.NoError(t, err)
		_, err = c.Read(b)
		assert.Equal(t, io.EOF, err)
	}()

	c, err := cm.Dial()
	assert.NoError(t, err)
	_, err = c.Write([]byte("PING"))
	assert.NoError(t, err)
	err = c.Close()
	assert.NoError(t, err)
	assert.NoError(t, err)
	_, err = c.Write([]byte("PING"))
	assert.Equal(t, io.EOF, err)

	wg.Wait()
}

func TestChannelServerClose(t *testing.T) {
	sm, cm := newServerAndClient()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := sm.Accept()
		assert.NoError(t, err)
		if err != nil {
			return
		}
		b := make([]byte, 4)
		_, err = c.Read(b)
		assert.NoError(t, err)
		err = c.Close()
		assert.NoError(t, err)
	}()

	c, err := cm.Dial()
	assert.NoError(t, err)
	_, err = c.Write([]byte("PING"))
	assert.NoError(t, err)
	b := make([]byte, 4)
	_, err = c.Read(b)
	assert.Equal(t, io.EOF, err)

	wg.Wait()
}

func TestMultiplexingServerClientPingPong(t *testing.T) {
	clients := 100
	packets := 100

	sm, cm := newServerAndClient()
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < clients; i++ {
			c, err := sm.Accept()
			assert.NoError(t, err)
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < packets; i++ {
					msg, id, err := readpacket(c)
					assert.NoError(t, err)
					assert.Equal(t, msg, fmt.Sprintf("PING%04d", i))
					err = writepacket(c, fmt.Sprintf("PONG%04d", i), id)
					assert.NoError(t, err)
				}
			}()
		}
	}()

	for i := 0; i < clients; i++ {
		c, err := cm.Dial()
		assert.NoError(t, err)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := uint32(0); i < uint32(packets); i++ {
				err = writepacket(c, fmt.Sprintf("PING%04d", i), i)
				assert.NoError(t, err)
				msg, id, err := readpacket(c)
				assert.NoError(t, err)
				assert.Equal(t, fmt.Sprintf("PONG%04d", i), msg)
				assert.Equal(t, id, i)
			}
		}()
	}

	wg.Wait()

	sm.Close()
}

func ExampleServer() {
	ln, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal(err)
	}

	// Network accept loop.
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}

		// Multiplexer accept loop.
		go func(conn net.Conn) {
			mx := MultiplexedServer(conn)
			for {
				c, err := mx.Accept()
				if err != nil {
					log.Fatal(err)
				}

				// Channel handler.
				go func(c *Channel) {
					defer c.Close()

					// Read "hello" from client.
					b := make([]byte, 5)
					_, err := c.Read(b)
					if err != nil {
						log.Fatal(err)
					}

					fmt.Printf("Received: %s\n", b)
				}(c)
			}
		}(conn)
	}
}

func ExampleClient() {
	conn, err := net.Dial("tcp", "127.0.0.1:1234")
	if err != nil {
		log.Fatal(err)
	}
	mx := MultiplexedClient(conn)

	// Create 10000 multiplexed connections over the socket and send "hello".
	for i := 0; i < 10000; i++ {
		go func() {
			c, err := mx.Dial()
			if err != nil {
				log.Fatal(err)
			}
			defer c.Close()

			_, err = c.Write([]byte("hello"))
			if err != nil {
				log.Fatal(err)
			}
		}()
	}
}
