// Package protocol implements the CN0261 wire formats: the Lambda TCP
// transport (byte-subtraction cipher with two static tables), the NX transport
// used by the relay, and the little-endian packet field codec.
package protocol

import (
	"errors"
	"fmt"
	"math"
	"unicode/utf16"
)

// Packet is one Lambda/NX message: a 16-bit message id and its payload.
type Packet struct {
	ID      uint16
	Payload []byte
}

var (
	// ErrTruncated reports a payload that ended in the middle of a field.
	ErrTruncated = errors.New("protocol: packet truncated")
	// ErrTrailing reports unexpected trailing bytes.
	ErrTrailing = errors.New("protocol: unexpected trailing fields")
	// ErrInvalidPacket reports a semantically invalid field value.
	ErrInvalidPacket = errors.New("protocol: invalid packet")
)

// Reader decodes the little-endian field layout used by the client.
type Reader struct {
	buf []byte
	pos int
}

// NewReader wraps a payload.
func NewReader(buf []byte) *Reader { return &Reader{buf: buf} }

// Remaining reports how many bytes are still unread.
func (r *Reader) Remaining() int { return len(r.buf) - r.pos }

// Byte reads one byte.
func (r *Reader) Byte() (uint8, error) {
	if r.Remaining() < 1 {
		return 0, fmt.Errorf("%w: 8-bit field", ErrTruncated)
	}
	v := r.buf[r.pos]
	r.pos++
	return v, nil
}

// Short reads a little-endian 16-bit field.
func (r *Reader) Short() (uint16, error) {
	if r.Remaining() < 2 {
		return 0, fmt.Errorf("%w: 16-bit field", ErrTruncated)
	}
	v := uint16(r.buf[r.pos]) | uint16(r.buf[r.pos+1])<<8
	r.pos += 2
	return v, nil
}

// Unsigned reads a little-endian 32-bit field.
func (r *Reader) Unsigned() (uint32, error) {
	if r.Remaining() < 4 {
		return 0, fmt.Errorf("%w: 32-bit field", ErrTruncated)
	}
	v := uint32(r.buf[r.pos]) | uint32(r.buf[r.pos+1])<<8 |
		uint32(r.buf[r.pos+2])<<16 | uint32(r.buf[r.pos+3])<<24
	r.pos += 4
	return v, nil
}

// Boolean reads the four-byte boolean encoding used on the wire.
func (r *Reader) Boolean() (bool, error) {
	v, err := r.Unsigned()
	if err != nil {
		return false, err
	}
	if v > 1 {
		return false, fmt.Errorf("%w: boolean must be zero or one", ErrInvalidPacket)
	}
	return v != 0, nil
}

// String reads a UTF-16LE string with a 32-bit code-unit count.
func (r *Reader) String(maxUnits int) ([]uint16, error) {
	length, err := r.Unsigned()
	if err != nil {
		return nil, err
	}
	if int(length) > maxUnits || int(length) > r.Remaining()/2 {
		return nil, fmt.Errorf("%w: string length %d is invalid", ErrInvalidPacket, length)
	}
	out := make([]uint16, length)
	for i := range out {
		out[i] = uint16(r.buf[r.pos]) | uint16(r.buf[r.pos+1])<<8
		r.pos += 2
	}
	return out, nil
}

// Skip discards n bytes.
func (r *Reader) Skip(n int) error {
	if r.Remaining() < n {
		return fmt.Errorf("%w: %d bytes", ErrTruncated, n)
	}
	r.pos += n
	return nil
}

// RequireEnd fails when bytes are left over.
func (r *Reader) RequireEnd() error {
	if r.Remaining() != 0 {
		return fmt.Errorf("%w: %d bytes left", ErrTrailing, r.Remaining())
	}
	return nil
}

// Writer encodes the little-endian field layout used by the client.
type Writer struct {
	buf []byte
}

// NewWriter returns an empty writer.
func NewWriter() *Writer { return &Writer{} }

// Byte appends one byte.
func (w *Writer) Byte(v uint8) { w.buf = append(w.buf, v) }

// Short appends a little-endian 16-bit field.
func (w *Writer) Short(v uint16) { w.buf = append(w.buf, byte(v), byte(v>>8)) }

// Unsigned appends a little-endian 32-bit field.
func (w *Writer) Unsigned(v uint32) {
	w.buf = append(w.buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

// Uint64 appends a little-endian 64-bit field as two 32-bit halves.
func (w *Writer) Uint64(v uint64) {
	w.Unsigned(uint32(v))
	w.Unsigned(uint32(v >> 32))
}

// Boolean appends the four-byte boolean encoding.
func (w *Writer) Boolean(v bool) {
	if v {
		w.Unsigned(1)
		return
	}
	w.Unsigned(0)
}

// Double appends an IEEE-754 double as low/high 32-bit halves.
func (w *Writer) Double(v float64) {
	bits := math.Float64bits(v)
	w.Uint64(bits)
}

// String appends a UTF-16LE string with its 32-bit code-unit count.
func (w *Writer) String(v []uint16) {
	w.Unsigned(uint32(len(v)))
	for _, unit := range v {
		w.buf = append(w.buf, byte(unit), byte(unit>>8))
	}
}

// Bytes appends raw bytes.
func (w *Writer) Bytes(v []byte) { w.buf = append(w.buf, v...) }

// Payload returns the accumulated bytes.
func (w *Writer) Payload() []byte { return w.buf }

// Endpoint selects the cipher start positions.
type Endpoint int

// Cipher endpoints. The server receives with {0,1} and sends with {5,3}.
const (
	EndpointServer Endpoint = iota
	EndpointClient
)

const (
	// LambdaHeaderSize is the fixed Lambda packet header size.
	LambdaHeaderSize = 5
	// LambdaMaxPayload is the largest Lambda payload.
	LambdaMaxPayload = 65535
	// LambdaGreetingSize is the greeting frame length the client expects.
	LambdaGreetingSize = 258
)

var lambdaFirstTable = [49]byte{
	0x5F, 0x3E, 0x32, 0x72, 0xC1, 0x18, 0x46, 0x87, 0x5D, 0x2E,
	0x29, 0x4E, 0x68, 0x79, 0x86, 0x6C, 0xB5, 0x8D, 0x56, 0xF1,
	0x38, 0xC1, 0x48, 0x42, 0x5F, 0x0C, 0xCD, 0x80, 0xE8, 0xE2,
	0xFE, 0xF0, 0x42, 0x5F, 0xF7, 0x59, 0x7E, 0x5F, 0xA5, 0x8D,
	0x3D, 0x76, 0xE7, 0x11, 0x41, 0x62, 0x67, 0x59, 0x7F,
}

var lambdaSecondTable = [24]byte{
	0x23, 0x00, 0x24, 0x00, 0x25, 0x00, 0x73, 0x00, 0x65, 0x00, 0x72, 0x00,
	0x74, 0x00, 0x33, 0x00, 0x34, 0x00, 0x25, 0x00, 0x77, 0x00, 0x65, 0x00,
}

// maskPeriod is the period of the Lambda stream mask: the two tables advance
// with coprime lengths (49 and 24), so the combined sequence repeats every
// 49*24 bytes. One precomputed period turns the per-byte table walk into a bulk
// add over the whole frame.
const maskPeriod = 49 * 24

var lambdaReceiveMask [maskPeriod]byte
var lambdaSendMask [maskPeriod]byte

var errLambdaStreamFailed = errors.New("protocol: lambda stream already failed")

func fillMaskTable(table *[maskPeriod]byte, first, second int) {
	for i := 0; i < maskPeriod; i++ {
		table[i] = lambdaFirstTable[first] + lambdaSecondTable[second]
		first = (first + 1) % len(lambdaFirstTable)
		second = (second + 1) % len(lambdaSecondTable)
	}
}

func init() {
	fillMaskTable(&lambdaReceiveMask, 0, 1)
	fillMaskTable(&lambdaSendMask, 5, 3)
}

// addMask adds the stream mask to dst and returns the next offset.
func addMask(dst []byte, mask *[maskPeriod]byte, offset int) int {
	for i := 0; i < len(dst); {
		chunk := len(dst) - i
		if rest := maskPeriod - offset; chunk > rest {
			chunk = rest
		}
		block, masks := dst[i:i+chunk], mask[offset:offset+chunk]
		for j := range block {
			block[j] += masks[j]
		}
		i += chunk
		if offset += chunk; offset == maskPeriod {
			offset = 0
		}
	}
	return offset
}

// subMask subtracts the stream mask from dst and returns the next offset.
func subMask(dst []byte, mask *[maskPeriod]byte, offset int) int {
	for i := 0; i < len(dst); {
		chunk := len(dst) - i
		if rest := maskPeriod - offset; chunk > rest {
			chunk = rest
		}
		block, masks := dst[i:i+chunk], mask[offset:offset+chunk]
		for j := range block {
			block[j] -= masks[j]
		}
		i += chunk
		if offset += chunk; offset == maskPeriod {
			offset = 0
		}
	}
	return offset
}

// LambdaCodec decodes and encodes the CN0261 Lambda transport. One codec
// instance belongs to one connection: the cipher offsets span packet boundaries.
type LambdaCodec struct {
	recvMask   *[maskPeriod]byte
	sendMask   *[maskPeriod]byte
	recvOffset int
	sendOffset int
	pending    []byte
	failed     bool
}

// NewLambdaCodec builds a codec for the given endpoint.
func NewLambdaCodec(endpoint Endpoint) *LambdaCodec {
	if endpoint == EndpointServer {
		return &LambdaCodec{recvMask: &lambdaReceiveMask, sendMask: &lambdaSendMask}
	}
	return &LambdaCodec{recvMask: &lambdaSendMask, sendMask: &lambdaReceiveMask}
}

// Greeting returns the fixed 258-byte greeting frame: a zero buffer whose first
// byte is 2, which the client's ServerConnection::OnGreeting accepts.
func Greeting() []byte {
	frame := make([]byte, LambdaGreetingSize)
	frame[0] = 2
	return frame
}

func headerWord(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }

// Feed consumes raw bytes and returns every complete packet. The incoming bytes
// are decrypted in bulk up front; parsing then walks a cursor over the pending
// buffer instead of appending byte by byte.
func (c *LambdaCodec) Feed(data []byte) ([]Packet, error) {
	if c.failed {
		return nil, errLambdaStreamFailed
	}
	if len(data) == 0 {
		return nil, nil
	}
	start := len(c.pending)
	c.pending = append(c.pending, data...)
	c.recvOffset = subMask(c.pending[start:], c.recvMask, c.recvOffset)
	return c.drain()
}

// drain extracts every complete packet from the pending buffer.
func (c *LambdaCodec) drain() ([]Packet, error) {
	var packets []Packet
	consumed := 0
	for len(c.pending)-consumed >= LambdaHeaderSize {
		frame := c.pending[consumed:]
		total := LambdaHeaderSize + int(headerWord(frame))
		if len(frame) < total {
			break
		}
		var checksum byte
		for _, b := range frame[LambdaHeaderSize:total] {
			checksum ^= b
		}
		if checksum != frame[4] {
			c.failed = true
			c.pending = c.pending[:0]
			return nil, fmt.Errorf("%w: lambda checksum mismatch", ErrInvalidPacket)
		}
		packets = append(packets, Packet{
			ID:      headerWord(frame[2:]),
			Payload: append([]byte(nil), frame[LambdaHeaderSize:total]...),
		})
		consumed += total
	}
	if consumed > 0 {
		c.pending = c.pending[:copy(c.pending, c.pending[consumed:])]
	}
	return packets, nil
}

// Encode frames one packet with the send cipher.
func (c *LambdaCodec) Encode(p Packet) ([]byte, error) { return c.EncodeInto(nil, p) }

// EncodeInto frames one packet into buf when it has enough capacity, so callers
// can reuse a scratch buffer per connection instead of allocating every frame.
func (c *LambdaCodec) EncodeInto(buf []byte, p Packet) ([]byte, error) {
	if len(p.Payload) > LambdaMaxPayload {
		return nil, fmt.Errorf("%w: payload exceeds 65535 bytes", ErrInvalidPacket)
	}
	size := LambdaHeaderSize + len(p.Payload)
	if cap(buf) < size {
		buf = make([]byte, size)
	} else {
		buf = buf[:size]
	}
	buf[0] = byte(len(p.Payload))
	buf[1] = byte(len(p.Payload) >> 8)
	buf[2] = byte(p.ID)
	buf[3] = byte(p.ID >> 8)
	copy(buf[LambdaHeaderSize:], p.Payload)
	var checksum byte
	for _, b := range buf[LambdaHeaderSize:] {
		checksum ^= b
	}
	buf[4] = checksum
	c.sendOffset = addMask(buf, c.sendMask, c.sendOffset)
	return buf, nil
}

var nxKeyTable = [255]byte{
	0x8A, 0x0C, 0x85, 0xF3, 0xCC, 0x82, 0x5F, 0xC1, 0x92, 0x56, 0x57, 0x6A,
	0xAA, 0x03, 0xF0, 0xBA, 0x70, 0xE7, 0x87, 0x7A, 0xBB, 0x38, 0x84, 0x9E,
	0x4C, 0x18, 0x99, 0x78, 0x50, 0x4D, 0xC5, 0x5A, 0x0F, 0x95, 0x5B, 0xF2,
	0x5A, 0x99, 0x34, 0xEA, 0x49, 0x1A, 0xC4, 0x89, 0x64, 0xB5, 0xD3, 0xD7,
	0x39, 0x72, 0xEA, 0x18, 0x18, 0xED, 0x60, 0x06, 0x53, 0x51, 0xFD, 0xAC,
	0x56, 0x7A, 0x20, 0xD6, 0x65, 0x72, 0x63, 0xD2, 0xDB, 0x12, 0x35, 0x9C,
	0xC0, 0x10, 0x52, 0xA9, 0xF7, 0x3D, 0x90, 0x57, 0xFB, 0xA8, 0x89, 0x4C,
	0x55, 0x0A, 0x68, 0x43, 0x42, 0x92, 0x8F, 0x8B, 0xCB, 0x09, 0x41, 0x3E,
	0x2D, 0xDE, 0x87, 0x76, 0x4C, 0x93, 0x0C, 0xF8, 0x37, 0x11, 0x12, 0xB1,
	0xD3, 0xEC, 0xCC, 0x6E, 0xBA, 0x21, 0x3F, 0xA2, 0x72, 0x2F, 0xD9, 0xC0,
	0x7C, 0x4F, 0x92, 0x63, 0xE2, 0x85, 0x22, 0xCF, 0x4C, 0xCC, 0x17, 0x8F,
	0x30, 0x38, 0x08, 0x30, 0x36, 0x2B, 0xFB, 0x5B, 0xEA, 0x8D, 0x50, 0x66,
	0x3C, 0xA4, 0xCC, 0x7A, 0x75, 0x70, 0x96, 0x7B, 0x29, 0xAF, 0x32, 0x59,
	0xCF, 0x71, 0xDA, 0xDC, 0xEB, 0xDF, 0x44, 0x29, 0x0B, 0x11, 0x9D, 0x8A,
	0x06, 0x20, 0x6B, 0xB3, 0xB4, 0x5B, 0x06, 0x19, 0x0D, 0x34, 0x56, 0x05,
	0xE2, 0xD8, 0xA1, 0x05, 0x37, 0x8E, 0x7A, 0xE4, 0x4B, 0x45, 0xA9, 0xCA,
	0xF8, 0x70, 0x24, 0x9F, 0x67, 0x1F, 0x61, 0xE2, 0xB4, 0xE4, 0xA0, 0x9D,
	0x2A, 0x06, 0x7E, 0xF9, 0x7B, 0x02, 0x65, 0x4B, 0xC9, 0xE2, 0xA4, 0x01,
	0xD3, 0x7B, 0xDC, 0xD1, 0x92, 0xE7, 0x70, 0x16, 0x24, 0x0B, 0xB3, 0xA3,
	0xD2, 0xDA, 0x8F, 0xAA, 0x95, 0x24, 0xBD, 0xCD, 0xCA, 0x32, 0xEC, 0x0B,
	0x1A, 0x73, 0x64, 0xA5, 0x36, 0x81, 0xED, 0xAB, 0xF5, 0x3A, 0xAE, 0x40,
	0xE5, 0x3A, 0xB4,
}

const (
	// NxHeaderSize is the NX packet header size.
	NxHeaderSize = 4
	// NxMaxPayload is the largest NX payload.
	NxMaxPayload = 65531
)

// NxCodec implements the relay's NX transport.
type NxCodec struct {
	transform bool
	pending   []byte
}

// NewNxCodec builds a codec; transform selects the XOR key schedule.
func NewNxCodec(transform bool) *NxCodec { return &NxCodec{transform: transform} }

func nxTransform(payload []byte, messageID uint16) {
	key := nxKeyTable[(len(payload)+int(messageID))%len(nxKeyTable)]
	for i := range payload {
		payload[i] ^= byte(int(key)%(i+1) + int(key))
	}
}

// Feed consumes raw bytes and returns every complete packet.
func (c *NxCodec) Feed(data []byte) ([]Packet, error) {
	var packets []Packet
	for len(data) > 0 {
		if len(c.pending) < NxHeaderSize {
			n := min(NxHeaderSize-len(c.pending), len(data))
			c.pending = append(c.pending, data[:n]...)
			data = data[n:]
			if len(c.pending) < NxHeaderSize {
				break
			}
		}
		length := int(headerWord(c.pending))
		if length > NxMaxPayload {
			c.pending = c.pending[:0]
			return nil, fmt.Errorf("%w: nx length overflows", ErrInvalidPacket)
		}
		total := NxHeaderSize + length
		if n := min(total-len(c.pending), len(data)); n > 0 {
			c.pending = append(c.pending, data[:n]...)
			data = data[n:]
		}
		if len(c.pending) < total {
			break
		}
		payload := append([]byte(nil), c.pending[NxHeaderSize:]...)
		id := headerWord(c.pending[2:])
		if c.transform {
			nxTransform(payload, id)
		}
		packets = append(packets, Packet{ID: id, Payload: payload})
		c.pending = c.pending[:0]
	}
	return packets, nil
}

// Encode frames one NX packet.
func (c *NxCodec) Encode(p Packet) ([]byte, error) {
	if len(p.Payload) > NxMaxPayload {
		return nil, fmt.Errorf("%w: nx payload too large", ErrInvalidPacket)
	}
	frame := make([]byte, 0, NxHeaderSize+len(p.Payload))
	frame = append(frame, byte(len(p.Payload)), byte(len(p.Payload)>>8),
		byte(p.ID), byte(p.ID>>8))
	frame = append(frame, p.Payload...)
	if c.transform {
		nxTransform(frame[NxHeaderSize:], p.ID)
	}
	return frame, nil
}

// Reset drops any partial packet.
func (c *NxCodec) Reset() { c.pending = c.pending[:0] }

// U16String converts a Go string to the client's UTF-16 code units.
func U16String(s string) []uint16 { return utf16.Encode([]rune(s)) }

// FromU16 converts client UTF-16 code units to a Go string.
func FromU16(units []uint16) string { return string(utf16.Decode(units)) }
