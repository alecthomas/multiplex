# Multiplexed streams for Go [![Build Status](https://travis-ci.org/alecthomas/multiplex.png)](https://travis-ci.org/alecthomas/multiplex)

Package multiplex provides multiplexed streams over a single underlying
transport `io.ReadWriteCloser`.

Any system that requires a large number of independent TCP connections
could benefit from this package, by instead having each client maintain a
single multiplexed connection. There is essentially very little cost to
creating new channels, or maintaining a large number of open channels.
Ideal for long term waiting.

An interesting side-effect of this multiplexing is that once the underlying
connection has been established, each end of the connection can both
`Accept()` and `Dial()`. This allows for elegant push notifications and
other interesting approaches.

### Documentation

Can be found  on [godoc.org](http://godoc.org/github.com/alecthomas/multiplex) or below.

### Example Server

	ln, err := net.Listen("tcp", ":1234")
	for {
	    conn, err := ln.Accept()
	    go func(conn net.Conn) {
	        mx := multiplex.MultiplexedServer(conn)
	        for {
	            c, err := mx.Accept()
	            go handleConnection(c)
	        }
	    }()
	}

### Example Client

Connect to a server with a single TCP connection, then create 10K channels
over it and write "hello" to each.

	conn, err := net.Dial("tcp", "127.0.0.1:1234")
	mx := multiplex.MultiplexedClient(conn)

	for i := 0; i < 10000; i++ {
	    go func() {
	        c, err := mx.Dial()
	        n, err := c.Write([]byte("hello"))
	        c.Close()
	    }()
	}

## Usage

```go
const (
	SYN = 1 << iota
	RST = 1 << iota
)
```
Packet flags.

```go
const (
	// FragmentSize (in bytes) of packet fragments.
	FragmentSize = 1024
)
```

```go
var (
	// ErrInvalidChannel is returned when an attempt is made to write to an invalid channel.
	ErrInvalidChannel = errors.New("invalid channel")
)
```

#### type Channel

```go
type Channel struct {
}
```

A Channel managed by the multiplexer.

#### func (*Channel) Close

```go
func (c *Channel) Close() error
```
Close a multiplexed channel.

#### func (*Channel) Read

```go
func (c *Channel) Read(b []byte) (int, error)
```
Read bytes from a multiplexed channel.

#### func (*Channel) Write

```go
func (c *Channel) Write(b []byte) (int, error)
```
Write bytes to a multiplexed channel. The underlying implementation will
fragment the payload into FragmentSize chunks to prevent starvation of other
channels.

#### type MultiplexedStream

```go
type MultiplexedStream struct {
}
```


#### func  MultiplexedClient

```go
func MultiplexedClient(conn io.ReadWriteCloser) *MultiplexedStream
```
MultiplexedClient creates a new multiplexed client-side stream.

#### func  MultiplexedServer

```go
func MultiplexedServer(conn io.ReadWriteCloser) *MultiplexedStream
```
MultiplexedServer creates a new multiplexed server-side stream.

#### func (*MultiplexedStream) Accept

```go
func (m *MultiplexedStream) Accept() (*Channel, error)
```

#### func (*MultiplexedStream) Close

```go
func (m *MultiplexedStream) Close() error
```

#### func (*MultiplexedStream) Dial

```go
func (m *MultiplexedStream) Dial() (*Channel, error)
```
Dial the remote end, creating a new multiplexed channel.
