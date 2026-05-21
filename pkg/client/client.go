package client

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Options configures the Synapse Cache client.
type Options struct {
	Addr        string
	Password    string
	MaxConns    int
	DialTimeout time.Duration
}

// Client is the Synapse Cache Go client.
type Client struct {
	opts Options
	pool chan net.Conn
	mu   sync.Mutex
}

// New creates a new client and dials the server.
func New(opts Options) (*Client, error) {
	if opts.MaxConns <= 0 {
		opts.MaxConns = 1
	}
	if opts.DialTimeout <= 0 {
		opts.DialTimeout = 5 * time.Second
	}

	c := &Client{
		opts: opts,
		pool: make(chan net.Conn, opts.MaxConns),
	}

	// Pre-fill pool with one connection to verify connectivity
	conn, err := c.dial()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", opts.Addr, err)
	}
	c.pool <- conn

	return c, nil
}

// Close closes all pooled connections.
func (c *Client) Close() error {
	close(c.pool)
	for conn := range c.pool {
		conn.Close()
	}
	return nil
}

// dial creates a new TCP connection and authenticates if a password is set.
func (c *Client) dial() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", c.opts.Addr, c.opts.DialTimeout)
	if err != nil {
		return nil, err
	}

	if c.opts.Password != "" {
		conn.SetDeadline(time.Now().Add(c.opts.DialTimeout))
		fmt.Fprintf(conn, "AUTH %s\r\n", c.opts.Password)
		
		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("auth failed to read response: %v", err)
		}
		if !strings.HasPrefix(line, "+OK") {
			conn.Close()
			return nil, fmt.Errorf("auth failed: %s", strings.TrimSpace(line))
		}
		conn.SetDeadline(time.Time{}) // reset deadline
	}

	return conn, nil
}

// getConn retrieves a connection from the pool or creates a new one.
func (c *Client) getConn(ctx context.Context) (net.Conn, error) {
	select {
	case conn := <-c.pool:
		if conn != nil {
			return conn, nil
		}
	default:
	}
	return c.dial()
}

// returnConn returns a connection to the pool.
func (c *Client) returnConn(conn net.Conn) {
	select {
	case c.pool <- conn:
	default:
		conn.Close() // pool full, discard
	}
}

// RawExec sends a raw command and returns the raw response lines.
func (c *Client) RawExec(ctx context.Context, cmd string) (string, error) {
	conn, err := c.getConn(ctx)
	if err != nil {
		return "", err
	}

	deadline, ok := ctx.Deadline()
	if ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(5 * time.Second))
	}

	_, err = fmt.Fprint(conn, cmd+"\r\n")
	if err != nil {
		conn.Close()
		return "", fmt.Errorf("write error: %w", err)
	}

	reader := bufio.NewReader(conn)
	resp, err := readResponse(reader)
	if err != nil {
		conn.Close()
		return "", err
	}

	c.returnConn(conn)
	return resp, nil
}

// readResponse reads a single RESP response.
func readResponse(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")

	if len(line) == 0 {
		return "", fmt.Errorf("empty response")
	}

	prefix := line[0]
	body := line[1:]

	switch prefix {
	case '+':
		return body, nil
	case '-':
		return "", fmt.Errorf("server error: %s", body)
	case ':':
		return body, nil
	case '$':
		length, err := strconv.Atoi(body)
		if err != nil {
			return "", fmt.Errorf("invalid bulk string length: %s", body)
		}
		if length == -1 {
			return "", ErrNil
		}
		// Read the data + \r\n
		data := make([]byte, length+2)
		_, err = reader.Read(data)
		if err != nil {
			return "", err
		}
		return string(data[:length]), nil
	case '*':
		// Array — read recursively
		count, err := strconv.Atoi(body)
		if err != nil {
			return "", fmt.Errorf("invalid array count: %s", body)
		}
		if count <= 0 {
			return "", nil
		}
		var items []string
		for i := 0; i < count; i++ {
			item, err := readResponse(reader)
			if err != nil {
				return "", err
			}
			items = append(items, item)
		}
		return strings.Join(items, "\n"), nil
	default:
		return line, nil
	}
}

// --- Error types ---

// ErrNil is returned when a key or vector is not found.
var ErrNil = fmt.Errorf("nil")

// --- Key-Value Operations ---

// Set stores a key-value pair with optional TTL.
func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	cmd := fmt.Sprintf("SET %s %s", key, value)
	if ttl > 0 {
		cmd += fmt.Sprintf(" EX %d", int(ttl.Seconds()))
	}
	_, err := c.RawExec(ctx, cmd)
	return err
}

// Get retrieves a value by key. Returns ErrNil if not found.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.RawExec(ctx, fmt.Sprintf("GET %s", key))
}

// Del deletes keys and returns the count deleted.
func (c *Client) Del(ctx context.Context, keys ...string) (int, error) {
	resp, err := c.RawExec(ctx, fmt.Sprintf("DEL %s", strings.Join(keys, " ")))
	if err != nil {
		return 0, err
	}
	n, _ := strconv.Atoi(resp)
	return n, nil
}

// --- Vector Operations ---

// VSetArgs holds arguments for the VSet command.
type VSetArgs struct {
	Namespace string
	ID        string
	Vector    []float32
	Metadata  map[string]string
}

// VSet stores a vector.
func (c *Client) VSet(ctx context.Context, args VSetArgs) error {
	parts := []string{"VSET", args.Namespace, args.ID, strconv.Itoa(len(args.Vector))}
	for _, f := range args.Vector {
		parts = append(parts, fmt.Sprintf("%f", f))
	}
	if len(args.Metadata) > 0 {
		parts = append(parts, "META")
		for k, v := range args.Metadata {
			parts = append(parts, k, v)
		}
	}
	_, err := c.RawExec(ctx, strings.Join(parts, " "))
	return err
}

// VSimilarityArgs holds arguments for the VSimilarity command.
type VSimilarityArgs struct {
	Namespace string
	Vector    []float32
	TopK      int
}

// SimilarityResult represents a single similarity search result.
type SimilarityResult struct {
	ID       string
	Score    float32
	Metadata map[string]string
}

// VSimilarity performs a top-K cosine similarity search.
func (c *Client) VSimilarity(ctx context.Context, args VSimilarityArgs) ([]SimilarityResult, error) {
	parts := []string{"VSIMILARITY", args.Namespace, strconv.Itoa(len(args.Vector))}
	for _, f := range args.Vector {
		parts = append(parts, fmt.Sprintf("%f", f))
	}
	parts = append(parts, "TOP", strconv.Itoa(args.TopK))

	resp, err := c.RawExec(ctx, strings.Join(parts, " "))
	if err != nil {
		return nil, err
	}

	if resp == "" {
		return nil, nil
	}

	// Parse response: lines of [id, score, meta...]
	// Each result is 3 lines: id, score, metadata
	lines := strings.Split(resp, "\n")
	var results []SimilarityResult

	for i := 0; i+2 < len(lines); i += 3 {
		score, _ := strconv.ParseFloat(lines[i+1], 32)
		result := SimilarityResult{
			ID:    lines[i],
			Score: float32(score),
		}
		// Parse metadata from the third line (if any)
		metaLine := lines[i+2]
		if metaLine != "" {
			metaParts := strings.Split(metaLine, "\n")
			if len(metaParts) >= 2 {
				result.Metadata = make(map[string]string)
				for j := 0; j+1 < len(metaParts); j += 2 {
					result.Metadata[metaParts[j]] = metaParts[j+1]
				}
			}
		}
		results = append(results, result)
	}

	return results, nil
}

// VCount returns the number of vectors in a namespace.
func (c *Client) VCount(ctx context.Context, namespace string) (int, error) {
	resp, err := c.RawExec(ctx, fmt.Sprintf("VCOUNT %s", namespace))
	if err != nil {
		return 0, err
	}
	n, _ := strconv.Atoi(resp)
	return n, nil
}

// Ping sends a PING command.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.RawExec(ctx, "PING")
	return err
}
