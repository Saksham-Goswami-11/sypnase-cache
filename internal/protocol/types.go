package protocol

import (
	"fmt"
	"strconv"
	"strings"
)

// Command represents a parsed client command.
type Command struct {
	Name string
	Args []string
}

// String reconstructs the command into a single line, quoting arguments if needed.
func (c Command) String() string {
	parts := make([]string, 0, len(c.Args)+1)
	parts = append(parts, c.Name)
	for _, arg := range c.Args {
		if strings.ContainsRune(arg, ' ') || strings.ContainsRune(arg, '"') {
			// Escape quotes
			arg = strings.ReplaceAll(arg, `"`, `\"`)
			parts = append(parts, `"`+arg+`"`)
		} else {
			parts = append(parts, arg)
		}
	}
	return strings.Join(parts, " ")
}

// Response types for the wire protocol.
// Prefixes: + simple string, - error, : integer, $ bulk string, * array

// SimpleString is a +OK style response.
type SimpleString struct {
	Value string
}

func (s *SimpleString) Serialize() []byte {
	return []byte("+" + s.Value + "\r\n")
}

// ErrorResponse is a -ERR style response.
type ErrorResponse struct {
	Message string
}

func (e *ErrorResponse) Serialize() []byte {
	return []byte("-ERR " + e.Message + "\r\n")
}

// IntegerResponse is a :42 style response.
type IntegerResponse struct {
	Value int
}

func (i *IntegerResponse) Serialize() []byte {
	return []byte(":" + strconv.Itoa(i.Value) + "\r\n")
}

// BulkString is a $<len>\r\n<data>\r\n response.
type BulkString struct {
	Value string
}

func (b *BulkString) Serialize() []byte {
	return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(b.Value), b.Value))
}

// NilBulk is a $-1 response indicating a missing key.
type NilBulk struct{}

func (n *NilBulk) Serialize() []byte {
	return []byte("$-1\r\n")
}

// ArrayResponse is a *<count> style response containing multiple items.
type ArrayResponse struct {
	Items []Response
}

func (a *ArrayResponse) Serialize() []byte {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%d\r\n", len(a.Items)))
	for _, item := range a.Items {
		sb.Write(item.Serialize())
	}
	return []byte(sb.String())
}

// Response is the interface all response types implement.
type Response interface {
	Serialize() []byte
}

// Pre-built responses for common cases.
var (
	RespOK   = &SimpleString{Value: "OK"}
	RespPong = &SimpleString{Value: "PONG"}
	RespNil  = &NilBulk{}
)
