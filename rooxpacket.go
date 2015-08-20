package ldapserver

import (
	"bufio"
	"errors"
	"fmt"

	roox "github.com/vjeantet/goldap/message"
)

func decodeMessage(bytes []byte) (ret roox.LDAPMessage, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = errors.New(fmt.Sprintf("%s", e))
		}
	}()
	zero := 0
	ret, err = roox.ReadLDAPMessage(roox.NewBytes(zero, bytes))
	return
}

func readLdapMessageBytes(br *bufio.Reader) (ret *[]byte, err error) {
	var bytes []byte
	var tagAndLength roox.TagAndLength
	tagAndLength, err = readTagAndLength(br, &bytes)
	if err != nil {
		return
	}
	readBytes(br, &bytes, tagAndLength.Length)
	return &bytes, err
}

// readTagAndLength parses an ASN.1 tag and length pair from a live connection
// into a byte slice. It returns the parsed data and the new offset. SET and
// SET OF (tag 17) are mapped to SEQUENCE and SEQUENCE OF (tag 16) since we
// don't distinguish between ordered and unordered objects in this code.
func readTagAndLength(conn *bufio.Reader, bytes *[]byte) (ret roox.TagAndLength, err error) {
	// offset = initOffset
	//b := bytes[offset]
	//offset++
	var b byte
	b, err = readBytes(conn, bytes, 1)
	if err != nil {
		return
	}
	ret.Class = int(b >> 6)
	ret.IsCompound = b&0x20 == 0x20
	ret.Tag = int(b & 0x1f)

	//	// If the bottom five bits are set, then the tag number is actually base 128
	//	// encoded afterwards
	//	if ret.tag == 0x1f {
	//		ret.tag, err = parseBase128Int(conn, bytes)
	//		if err != nil {
	//			return
	//		}
	//	}
	// We are expecting the LDAP sequence tag 0x30 as first byte
	if b != 0x30 {
		panic(fmt.Sprintf("Expecting 0x30 as first byte, but got %#x instead", b))
	}

	b, err = readBytes(conn, bytes, 1)
	if err != nil {
		return
	}
	if b&0x80 == 0 {
		// The length is encoded in the bottom 7 bits.
		ret.Length = int(b & 0x7f)
	} else {
		// Bottom 7 bits give the number of length bytes to follow.
		numBytes := int(b & 0x7f)
		if numBytes == 0 {
			err = roox.SyntaxError{"indefinite length found (not DER)"}
			return
		}
		ret.Length = 0
		for i := 0; i < numBytes; i++ {

			b, err = readBytes(conn, bytes, 1)
			if err != nil {
				return
			}
			if ret.Length >= 1<<23 {
				// We can't shift ret.length up without
				// overflowing.
				err = roox.StructuralError{"length too large"}
				return
			}
			ret.Length <<= 8
			ret.Length |= int(b)
			if ret.Length == 0 {
				// DER requires that lengths be minimal.
				err = roox.StructuralError{"superfluous leading zeros in length"}
				return
			}
		}
	}

	return
}

// Read "length" bytes from the connection
// Append the read bytes to "bytes"
// Return the last read byte
func readBytes(conn *bufio.Reader, bytes *[]byte, length int) (b byte, err error) {
	newbytes := make([]byte, length)
	n, err := conn.Read(newbytes)
	if n != length {
		fmt.Errorf("%d bytes read instead of %d", n, length)
	} else if err != nil {
		return
	}
	*bytes = append(*bytes, newbytes...)
	b = (*bytes)[len(*bytes)-1]
	return
}