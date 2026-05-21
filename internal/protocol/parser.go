package protocol

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

const maxLineLength = 1024 * 64 // 64KB max line length

// Parse reads one or more commands from the reader (supports pipelining).
// Returns the parsed commands and any error encountered.
func Parse(reader *bufio.Reader) ([]Command, error) {
	var commands []Command

	for {
		var lineBuf strings.Builder
		var err error
		for {
			b, errRead := reader.ReadByte()
			if errRead != nil {
				err = errRead
				break
			}
			lineBuf.WriteByte(b)
			
			// LFI / OOM MITIGATION: Enforce limit during read, not after
			if lineBuf.Len() > maxLineLength {
				return nil, fmt.Errorf("line exceeds maximum length of %d bytes", maxLineLength)
			}
			
			if b == '\n' {
				break
			}
		}

		line := lineBuf.String()
		if err != nil {
			if err == io.EOF {
				// If we have a partial line without \n, try to parse it
				line = strings.TrimSpace(line)
				if len(line) > 0 {
					cmd, parseErr := parseLine(line)
					if parseErr == nil {
						commands = append(commands, cmd)
					}
				}
				if len(commands) > 0 {
					return commands, nil
				}
				return nil, io.EOF
			}
			return commands, err
		}

		// Trim \r\n or \n
		line = strings.TrimRight(line, "\r\n")

		if len(line) == 0 {
			// Empty line — if we already have commands, return them (end of pipeline).
			// Otherwise, skip and continue reading.
			if len(commands) > 0 {
				return commands, nil
			}
			continue
		}

		cmd, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		commands = append(commands, cmd)

		// Check if there's more data buffered (pipelining).
		// If not, return what we have.
		if reader.Buffered() == 0 {
			break
		}
	}

	return commands, nil
}

// parseLine tokenizes a single command line into a Command struct.
func parseLine(line string) (Command, error) {
	tokens := tokenize(line)
	if len(tokens) == 0 {
		return Command{}, fmt.Errorf("empty command")
	}

	return Command{
		Name: strings.ToUpper(tokens[0]),
		Args: tokens[1:],
	}, nil
}

// tokenize splits a line by whitespace, respecting quoted strings and backslash escaping.
func tokenize(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)
	escaped := false

	for i := 0; i < len(line); i++ {
		ch := line[i]

		if escaped {
			switch ch {
			case 'n':
				current.WriteByte('\n')
			case 'r':
				current.WriteByte('\r')
			case 't':
				current.WriteByte('\t')
			default:
				current.WriteByte(ch)
			}
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		if inQuote {
			if ch == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(ch)
			}
		} else {
			if ch == '"' || ch == '\'' {
				inQuote = true
				quoteChar = ch
			} else if ch == ' ' || ch == '\t' {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			} else {
				current.WriteByte(ch)
			}
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
