// SPDX-License-Identifier: BSD-3-Clause
// SPDX-FileCopyrightText: Copyright 2021 The Go Language Server Authors

// Package fakenet fake implements of net.Conn.
package fakenet

import (
	"io"
	"net"
	"sync"
	"time"
)

type fakeAddr string

type fakeConn struct {
	name   string
	reader *connFeeder
	writer *connFeeder
	in     io.ReadCloser
	out    io.WriteCloser
}

// make sure a fakeCon implements the net.Conn interface.
var _ net.Conn = (*fakeConn)(nil)

// NewConn returns a net.Conn built on top of the supplied reader and writer.
//
// It decouples the read and write on the conn from the underlying stream
// to enable Close to abort ones that are in progress.
//
// It's primary use is to fake a network connection from stdin and stdout.
func NewConn(name string, in io.ReadCloser, out io.WriteCloser) net.Conn {
	c := &fakeConn{
		name:   name,
		reader: newFeeder(in.Read),
		writer: newFeeder(out.Write),
		in:     in,
		out:    out,
	}
	go c.reader.run()
	go c.writer.run()
	return c
}

// Read implements net.Conn.Read.
func (c *fakeConn) Read(b []byte) (n int, err error) { return c.reader.do(b) }

// Write implements net.Conn.Write.
func (c *fakeConn) Write(b []byte) (n int, err error) { return c.writer.do(b) }

// LocalAddr implements net.Conn.LocalAddr.
func (c *fakeConn) LocalAddr() net.Addr { return fakeAddr(c.name) }

// RemoteAddr implements net.Conn.RemoteAddr.
func (c *fakeConn) RemoteAddr() net.Addr { return fakeAddr(c.name) }

// SetDeadline implements net.Conn.SetDeadline.
func (c *fakeConn) SetDeadline(t time.Time) error { return nil }

// SetReadDeadline implements net.Conn.SetReadDeadline.
func (c *fakeConn) SetReadDeadline(t time.Time) error { return nil }

// SetWriteDeadline implements net.Conn.SetWriteDeadline.
func (c *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

// Network implements net.Conn.Network.
func (a fakeAddr) Network() string { return "fake" }

// String implements net.Conn.String.
func (a fakeAddr) String() string { return string(a) }

// Close implements net.Conn.Close.
func (c *fakeConn) Close() error {
	c.reader.close()
	c.writer.close()
	c.in.Close()
	c.out.Close()
	return nil
}

// connFeeder serializes calls to the source function (io.Reader.Read or
// io.Writer.Write) by delegating them to a channel. This also allows calls to
// be intercepted when the connection is closed, and cancelled early if the
// connection is closed while the calls are still outstanding.
type connFeeder struct {
	source func([]byte) (int, error)
	input  chan []byte
	result chan feedResult
	mu     sync.Mutex
	closed bool
	done   chan struct{}
}

type feedResult struct {
	n   int
	err error
}

func newFeeder(source func([]byte) (int, error)) *connFeeder {
	return &connFeeder{
		source: source,
		input:  make(chan []byte),
		result: make(chan feedResult),
		done:   make(chan struct{}),
	}
}

func (f *connFeeder) close() {
	f.mu.Lock()
	if !f.closed {
		f.closed = true
		close(f.done)
	}
	f.mu.Unlock()
}

func (f *connFeeder) do(b []byte) (n int, err error) {
	// send the request to the worker
	select {
	case f.input <- b:
	case <-f.done:
		return 0, io.EOF
	}
	// get the result from the worker
	select {
	case r := <-f.result:
		return r.n, r.err
	case <-f.done:
		return 0, io.EOF
	}
}

func (f *connFeeder) run() {
	var b []byte
	for {
		// wait for an input request
		select {
		case b = <-f.input:
		case <-f.done:
			return
		}
		// invoke the underlying method
		n, err := f.source(b)
		// send the result back to the requester
		select {
		case f.result <- feedResult{n: n, err: err}:
		case <-f.done:
			return
		}
	}
}
