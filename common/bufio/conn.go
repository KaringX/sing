package bufio

import (
	"context"
	"io"
	"net"
	"os"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/common/task"
)

type readOnlyReader struct {
	io.Reader
}

func (r *readOnlyReader) WriteTo(w io.Writer) (n int64, err error) {
	return Copy(w, r.Reader)
}

func needReadFromWrapper(dst io.ReaderFrom, src io.Reader) bool {
	_, isTCPConn := dst.(*net.TCPConn)
	if !isTCPConn {
		return false
	}
	switch src.(type) {
	case *net.TCPConn, *net.UnixConn, *os.File:
		return false
	default:
		return true
	}
}

func Copy(dst io.Writer, src io.Reader) (n int64, err error) {
	src = N.UnwrapReader(src)
	dst = N.UnwrapWriter(dst)
	if wt, ok := src.(io.WriterTo); ok {
		return wt.WriteTo(dst)
	}
	if rt, ok := dst.(io.ReaderFrom); ok {
		if needReadFromWrapper(rt, src) {
			src = &readOnlyReader{src}
		}
		return rt.ReadFrom(src)
	}
	return CopyExtended(NewExtendedWriter(dst), NewExtendedReader(src))
}

func CopyExtended(dst N.ExtendedWriter, src N.ExtendedReader) (n int64, err error) {
	unsafeSrc, srcUnsafe := common.Cast[N.ThreadSafeReader](src)
	_, dstUnsafe := common.Cast[N.ThreadUnsafeWriter](dst)
	if srcUnsafe {
		return CopyExtendedWithSrcBuffer(dst, unsafeSrc)
	} else if dstUnsafe {
		return CopyExtendedWithPool(dst, src)
	}

	_buffer := buf.StackNew()
	defer common.KeepAlive(_buffer)

	return CopyExtendedBuffer(dst, src, common.Dup(_buffer))
}

func CopyExtendedBuffer(dst N.ExtendedWriter, src N.ExtendedReader, buffer *buf.Buffer) (n int64, err error) {
	buffer.IncRef()
	defer buffer.DecRef()
	for {
		buffer.Reset()
		err = src.ReadBuffer(buffer)
		if err != nil {
			return
		}
		dataLen := buffer.Len()
		err = dst.WriteBuffer(buffer)
		if err != nil {
			return
		}
		n += int64(dataLen)
	}
}

func CopyExtendedWithSrcBuffer(dst N.ExtendedWriter, src N.ThreadSafeReader) (n int64, err error) {
	for {
		var buffer *buf.Buffer
		buffer, err = src.ReadBufferThreadSafe()
		if err != nil {
			return
		}
		dataLen := buffer.Len()
		err = dst.WriteBuffer(buffer)
		if err != nil {
			buffer.Release()
			return
		}
		n += int64(dataLen)
	}
}

func CopyExtendedWithPool(dst N.ExtendedWriter, src N.ExtendedReader) (n int64, err error) {
	for {
		buffer := buf.New()
		err = src.ReadBuffer(buffer)
		if err != nil {
			buffer.Release()
			return
		}
		dataLen := buffer.Len()
		err = dst.WriteBuffer(buffer)
		if err != nil {
			buffer.Release()
			return
		}
		n += int64(dataLen)
	}
}

func CopyConn(ctx context.Context, conn net.Conn, dest net.Conn) error {
	defer common.Close(conn, dest)
	err := task.Run(ctx, func() error {
		defer rw.CloseRead(conn)
		defer rw.CloseWrite(dest)
		return common.Error(Copy(dest, conn))
	}, func() error {
		defer rw.CloseRead(dest)
		defer rw.CloseWrite(conn)
		return common.Error(Copy(conn, dest))
	})
	return err
}

func CopyPacket(dst N.PacketWriter, src N.PacketReader) (n int64, err error) {
	unsafeSrc, srcUnsafe := common.Cast[N.ThreadSafePacketReader](src)
	_, dstUnsafe := common.Cast[N.ThreadUnsafeWriter](dst)
	if srcUnsafe {
		return CopyPacketWithSrcBuffer(dst, unsafeSrc)
	} else if dstUnsafe {
		return CopyPacketWithPool(dst, src)
	}

	_buffer := buf.StackNewPacket()
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	buffer.IncRef()
	defer buffer.DecRef()
	var destination M.Socksaddr
	for {
		buffer.Reset()
		destination, err = src.ReadPacket(buffer)
		if err != nil {
			return
		}
		dataLen := buffer.Len()
		err = dst.WritePacket(buffer, destination)
		if err != nil {
			return
		}
		n += int64(dataLen)
	}
}

func CopyPacketTimeout(dst N.PacketWriter, src N.TimeoutPacketReader, timeout time.Duration) (n int64, err error) {
	unsafeSrc, srcUnsafe := common.Cast[N.ThreadSafePacketReader](src)
	_, dstUnsafe := common.Cast[N.ThreadUnsafeWriter](dst)
	if srcUnsafe {
		return CopyPacketWithSrcBufferTimeout(dst, unsafeSrc, src, timeout)
	} else if dstUnsafe {
		return CopyPacketWithPoolTimeout(dst, src, timeout)
	}

	_buffer := buf.StackNewPacket()
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	buffer.IncRef()
	defer buffer.DecRef()
	var destination M.Socksaddr
	for {
		buffer.Reset()
		err = src.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return
		}
		destination, err = src.ReadPacket(buffer)
		if err != nil {
			return
		}
		dataLen := buffer.Len()
		err = dst.WritePacket(buffer, destination)
		if err != nil {
			return
		}
		n += int64(dataLen)
	}
}

func CopyPacketWithSrcBuffer(dest N.PacketWriter, src N.ThreadSafePacketReader) (n int64, err error) {
	var buffer *buf.Buffer
	var destination M.Socksaddr
	for {
		buffer, destination, err = src.ReadPacketThreadSafe()
		if err != nil {
			return
		}
		dataLen := buffer.Len()
		err = dest.WritePacket(buffer, destination)
		if err != nil {
			buffer.Release()
			return
		}
		n += int64(dataLen)
	}
}

func CopyPacketWithSrcBufferTimeout(dest N.PacketWriter, src N.ThreadSafePacketReader, tSrc N.TimeoutPacketReader, timeout time.Duration) (n int64, err error) {
	var buffer *buf.Buffer
	var destination M.Socksaddr
	for {
		err = tSrc.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return
		}
		buffer, destination, err = src.ReadPacketThreadSafe()
		if err != nil {
			return
		}
		dataLen := buffer.Len()
		err = dest.WritePacket(buffer, destination)
		if err != nil {
			buffer.Release()
			return
		}
		n += int64(dataLen)
	}
}

func CopyPacketWithPool(dest N.PacketWriter, src N.PacketReader) (n int64, err error) {
	var destination M.Socksaddr
	for {
		buffer := buf.New()
		destination, err = src.ReadPacket(buffer)
		if err != nil {
			buffer.Release()
			return
		}
		dataLen := buffer.Len()
		err = dest.WritePacket(buffer, destination)
		if err != nil {
			buffer.Release()
			return
		}
		n += int64(dataLen)
	}
}

func CopyPacketWithPoolTimeout(dest N.PacketWriter, src N.TimeoutPacketReader, timeout time.Duration) (n int64, err error) {
	var destination M.Socksaddr
	for {
		buffer := buf.New()
		err = src.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return
		}
		destination, err = src.ReadPacket(buffer)
		if err != nil {
			buffer.Release()
			return
		}
		dataLen := buffer.Len()
		err = dest.WritePacket(buffer, destination)
		if err != nil {
			buffer.Release()
			return
		}
		n += int64(dataLen)
	}
}

func CopyPacketConn(ctx context.Context, conn N.PacketConn, dest N.PacketConn) error {
	defer common.Close(conn, dest)
	return task.Any(ctx, func() error {
		return common.Error(CopyPacket(dest, conn))
	}, func() error {
		return common.Error(CopyPacket(conn, dest))
	})
}

func CopyPacketConnTimeout(ctx context.Context, conn N.PacketConn, dest N.PacketConn, timeout time.Duration) error {
	defer common.Close(conn, dest)
	return task.Any(ctx, func() error {
		return common.Error(CopyPacketTimeout(dest, conn, timeout))
	}, func() error {
		return common.Error(CopyPacketTimeout(conn, dest, timeout))
	})
}

func NewPacketConn(conn net.PacketConn) N.PacketConn {
	if udpConn, ok := conn.(*net.UDPConn); ok {
		return &ExtendedUDPConn{udpConn}
	} else {
		return &ExtendedPacketConn{udpConn}
	}
}

type ExtendedUDPConn struct {
	*net.UDPConn
}

func (w *ExtendedUDPConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	n, addr, err := w.ReadFromUDPAddrPort(buffer.FreeBytes())
	if err != nil {
		return M.Socksaddr{}, err
	}
	buffer.Truncate(n)
	return M.SocksaddrFromNetIP(addr), nil
}

func (w *ExtendedUDPConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	if destination.Family().IsFqdn() {
		udpAddr, err := net.ResolveUDPAddr("udp", destination.String())
		if err != nil {
			return err
		}
		return common.Error(w.UDPConn.WriteTo(buffer.Bytes(), udpAddr))
	}
	return common.Error(w.UDPConn.WriteToUDP(buffer.Bytes(), destination.UDPAddr()))
}

func (w *ExtendedUDPConn) Upstream() any {
	return w.UDPConn
}

type ExtendedPacketConn struct {
	net.PacketConn
}

func (w *ExtendedPacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	_, addr, err := ReadFrom(w, buffer)
	if err != nil {
		return M.Socksaddr{}, err
	}
	return M.SocksaddrFromNet(addr), err
}

func (w *ExtendedPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	return common.Error(w.WriteTo(buffer.Bytes(), destination.UDPAddr()))
}

func (w *ExtendedPacketConn) Upstream() any {
	return w.PacketConn
}

type BindPacketConn struct {
	net.PacketConn
	Addr net.Addr
}

func (c *BindPacketConn) Read(b []byte) (n int, err error) {
	n, _, err = c.ReadFrom(b)
	return
}

func (c *BindPacketConn) Write(b []byte) (n int, err error) {
	return c.WriteTo(b, c.Addr)
}

func (c *BindPacketConn) RemoteAddr() net.Addr {
	return c.Addr
}

func (c *BindPacketConn) Upstream() any {
	return c.PacketConn
}

type UnbindPacketConn struct {
	net.Conn
}

func (c *UnbindPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.Conn.Read(p)
	if err == nil {
		addr = c.RemoteAddr()
	}
	return
}

func (c *UnbindPacketConn) WriteTo(p []byte, _ net.Addr) (n int, err error) {
	return c.Write(p)
}

func (c *UnbindPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	_, err = buffer.ReadFrom(c.Conn)
	if err != nil {
		return
	}
	destination = M.SocksaddrFromNet(c.RemoteAddr())
	return
}

func (c *UnbindPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	return common.Error(c.Conn.Write(buffer.Bytes()))
}

func (c *UnbindPacketConn) Upstream() any {
	return c.Conn
}

type ExtendedReaderWrapper struct {
	io.Reader
}

func (r *ExtendedReaderWrapper) ReadBuffer(buffer *buf.Buffer) error {
	n, err := r.Read(buffer.FreeBytes())
	if err != nil {
		return err
	}
	buffer.Truncate(n)
	return nil
}

func (r *ExtendedReaderWrapper) Upstream() any {
	return r.Reader
}

func (r *ExtendedReaderWrapper) ReaderReplaceable() bool {
	return true
}

func NewExtendedReader(reader io.Reader) N.ExtendedReader {
	if r, ok := reader.(N.ExtendedReader); ok {
		return r
	}
	return &ExtendedReaderWrapper{reader}
}

type ExtendedWriterWrapper struct {
	io.Writer
}

func (w *ExtendedWriterWrapper) WriteBuffer(buffer *buf.Buffer) error {
	return common.Error(w.Write(buffer.Bytes()))
}

func (r *ExtendedWriterWrapper) Upstream() any {
	return r.Writer
}

func (r *ExtendedReaderWrapper) WriterReplaceable() bool {
	return true
}

func NewExtendedWriter(writer io.Writer) N.ExtendedWriter {
	if w, ok := writer.(N.ExtendedWriter); ok {
		return w
	}
	return &ExtendedWriterWrapper{writer}
}

type ExtendedConnWrapper struct {
	net.Conn
	reader N.ExtendedReader
	writer N.ExtendedWriter
}

func (w *ExtendedConnWrapper) ReadBuffer(buffer *buf.Buffer) error {
	return w.reader.ReadBuffer(buffer)
}

func (w *ExtendedConnWrapper) WriteBuffer(buffer *buf.Buffer) error {
	return w.writer.WriteBuffer(buffer)
}

func NewExtendedConn(conn net.Conn) N.ExtendedConn {
	if c, ok := conn.(N.ExtendedConn); ok {
		return c
	}
	return &ExtendedConnWrapper{
		Conn:   conn,
		reader: NewExtendedReader(conn),
		writer: NewExtendedWriter(conn),
	}
}
