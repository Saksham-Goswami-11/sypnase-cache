package protocol

import (
	"bufio"
	"strings"
	"testing"
)

func TestParseSingleCommand(t *testing.T) {
	input := "SET mykey myvalue\r\n"
	reader := bufio.NewReader(strings.NewReader(input))

	commands, err := Parse(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}
	if commands[0].Name != "SET" {
		t.Errorf("expected command name SET, got %s", commands[0].Name)
	}
	if len(commands[0].Args) != 2 || commands[0].Args[0] != "mykey" || commands[0].Args[1] != "myvalue" {
		t.Errorf("unexpected args: %v", commands[0].Args)
	}
}

func TestParsePing(t *testing.T) {
	input := "PING\r\n"
	reader := bufio.NewReader(strings.NewReader(input))

	commands, err := Parse(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}
	if commands[0].Name != "PING" {
		t.Errorf("expected PING, got %s", commands[0].Name)
	}
	if len(commands[0].Args) != 0 {
		t.Errorf("expected 0 args, got %d", len(commands[0].Args))
	}
}

func TestParsePipelinedCommands(t *testing.T) {
	input := "SET key1 val1\r\nGET key1\r\nDEL key1\r\n"
	reader := bufio.NewReader(strings.NewReader(input))

	commands, err := Parse(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(commands))
	}

	expected := []string{"SET", "GET", "DEL"}
	for i, cmd := range commands {
		if cmd.Name != expected[i] {
			t.Errorf("command %d: expected %s, got %s", i, expected[i], cmd.Name)
		}
	}
}

func TestParseCaseInsensitive(t *testing.T) {
	input := "ping\r\n"
	reader := bufio.NewReader(strings.NewReader(input))

	commands, err := Parse(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commands[0].Name != "PING" {
		t.Errorf("expected PING (uppercase), got %s", commands[0].Name)
	}
}

func TestParseQuotedStrings(t *testing.T) {
	input := `SET mykey "hello world"` + "\r\n"
	reader := bufio.NewReader(strings.NewReader(input))

	commands, err := Parse(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands[0].Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(commands[0].Args))
	}
	if commands[0].Args[1] != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", commands[0].Args[1])
	}
}

func TestParseEOFWithPartialLine(t *testing.T) {
	// No trailing \r\n — should still parse
	input := "PING"
	reader := bufio.NewReader(strings.NewReader(input))

	commands, err := Parse(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}
	if commands[0].Name != "PING" {
		t.Errorf("expected PING, got %s", commands[0].Name)
	}
}

func TestParseSetWithEX(t *testing.T) {
	input := "SET session:abc123 user_42 EX 3600\r\n"
	reader := bufio.NewReader(strings.NewReader(input))

	commands, err := Parse(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commands[0].Args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(commands[0].Args), commands[0].Args)
	}
	if commands[0].Args[2] != "EX" || commands[0].Args[3] != "3600" {
		t.Errorf("unexpected EX args: %v", commands[0].Args[2:])
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"SET key val", []string{"SET", "key", "val"}},
		{`SET key "hello world"`, []string{"SET", "key", "hello world"}},
		{`SET key "hello \"world\""`, []string{"SET", "key", `hello "world"`}},
		{`SET key "hello\nworld"`, []string{"SET", "key", "hello\nworld"}},
		{"  PING  ", []string{"PING"}},
		{"", nil},
		{"SET  key\t\tval", []string{"SET", "key", "val"}},
	}

	for _, tt := range tests {
		tokens := tokenize(tt.input)
		if len(tokens) != len(tt.expected) {
			t.Errorf("tokenize(%q): expected %d tokens, got %d: %v", tt.input, len(tt.expected), len(tokens), tokens)
			continue
		}
		for i := range tokens {
			if tokens[i] != tt.expected[i] {
				t.Errorf("tokenize(%q)[%d]: expected %q, got %q", tt.input, i, tt.expected[i], tokens[i])
			}
		}
	}
}

// TestResponseSerialization verifies all response types serialize correctly.
func TestResponseSerialization(t *testing.T) {
	tests := []struct {
		resp     Response
		expected string
	}{
		{&SimpleString{Value: "OK"}, "+OK\r\n"},
		{&SimpleString{Value: "PONG"}, "+PONG\r\n"},
		{&ErrorResponse{Message: "unknown command"}, "-ERR unknown command\r\n"},
		{&IntegerResponse{Value: 42}, ":42\r\n"},
		{&IntegerResponse{Value: 0}, ":0\r\n"},
		{&IntegerResponse{Value: -1}, ":-1\r\n"},
		{&BulkString{Value: "hello"}, "$5\r\nhello\r\n"},
		{&NilBulk{}, "$-1\r\n"},
		{
			&ArrayResponse{Items: []Response{
				&BulkString{Value: "key1"},
				&IntegerResponse{Value: 1},
			}},
			"*2\r\n$4\r\nkey1\r\n:1\r\n",
		},
	}

	for _, tt := range tests {
		got := string(tt.resp.Serialize())
		if got != tt.expected {
			t.Errorf("Serialize() = %q, want %q", got, tt.expected)
		}
	}
}

func TestParseOOMMitigation(t *testing.T) {
	// Create a payload larger than maxLineLength (64KB)
	largePayload := "SET key " + strings.Repeat("A", 65*1024) + "\r\n"
	reader := bufio.NewReader(strings.NewReader(largePayload))

	_, err := Parse(reader)
	if err == nil {
		t.Fatalf("expected error for payload exceeding max length, got nil")
	}

	if !strings.Contains(err.Error(), "exceeds maximum length") {
		t.Errorf("expected max length error, got: %v", err)
	}
}
