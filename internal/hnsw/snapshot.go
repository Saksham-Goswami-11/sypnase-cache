package hnsw

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
)

const (
	magicHNSW = "HNSW"
	version   = byte(1)
)

// Snapshot encodes the current state of the HNSW index into a binary format.
// Format:
// [Magic "HNSW" 4 bytes] [Version 1 byte]
// [Header: M, Mmax0, EfConstruction, Ef, currentMaxLayer, entryPoint (uint32 each)]
// [Node count (uint32)]
// For each node:
//   [ID uint32] [MaxLayer uint32]
//   [Vector length uint32] [Vector float32 data...]
//   [Metadata JSON string length uint32] [Metadata JSON string...]
//   For each layer from 0 to MaxLayer:
//     [Links count uint32] [Link IDs...]
// [CRC32 Checksum 4 bytes]
func (idx *Index) Snapshot(w io.Writer) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Use a buffer to calculate CRC32
	var buf bytes.Buffer

	// 1. Magic and Version
	buf.WriteString(magicHNSW)
	buf.WriteByte(version)

	// 2. Header
	err := binary.Write(&buf, binary.LittleEndian, []uint32{
		uint32(idx.M),
		uint32(idx.Mmax0),
		uint32(idx.EfConstruction),
		uint32(idx.Ef),
		uint32(idx.currentMaxLayer),
		idx.entryPoint,
	})
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// 3. Node count
	nodeCount := uint32(len(idx.nodes))
	if err := binary.Write(&buf, binary.LittleEndian, nodeCount); err != nil {
		return fmt.Errorf("failed to write node count: %w", err)
	}

	// 4. Nodes
	for _, node := range idx.nodes {
		node.mu.RLock()
		
		// ID and MaxLayer
		if err := binary.Write(&buf, binary.LittleEndian, node.ID); err != nil {
			node.mu.RUnlock()
			return err
		}
		maxLayer := uint32(len(node.Links) - 1)
		if err := binary.Write(&buf, binary.LittleEndian, maxLayer); err != nil {
			node.mu.RUnlock()
			return err
		}

		// Vector
		vecLen := uint32(len(node.Vector))
		if err := binary.Write(&buf, binary.LittleEndian, vecLen); err != nil {
			node.mu.RUnlock()
			return err
		}
		if err := binary.Write(&buf, binary.LittleEndian, node.Vector); err != nil {
			node.mu.RUnlock()
			return err
		}

		// Metadata
		metaBytes, err := json.Marshal(node.Metadata)
		if err != nil {
			node.mu.RUnlock()
			return fmt.Errorf("failed to marshal metadata for node %d: %w", node.ID, err)
		}
		metaLen := uint32(len(metaBytes))
		if err := binary.Write(&buf, binary.LittleEndian, metaLen); err != nil {
			node.mu.RUnlock()
			return err
		}
		buf.Write(metaBytes)

		// Links
		for l := 0; l <= int(maxLayer); l++ {
			links := node.Links[l]
			linkCount := uint32(len(links))
			if err := binary.Write(&buf, binary.LittleEndian, linkCount); err != nil {
				node.mu.RUnlock()
				return err
			}
			if err := binary.Write(&buf, binary.LittleEndian, links); err != nil {
				node.mu.RUnlock()
				return err
			}
		}
		node.mu.RUnlock()
	}

	// String mappings
	// Reverse Map
	revCount := uint32(len(idx.reverseMap))
	if err := binary.Write(&buf, binary.LittleEndian, revCount); err != nil {
		return err
	}
	for _, key := range idx.reverseMap {
		keyBytes := []byte(key)
		keyLen := uint32(len(keyBytes))
		if err := binary.Write(&buf, binary.LittleEndian, keyLen); err != nil {
			return err
		}
		buf.Write(keyBytes)
	}

	// Calculate CRC32
	data := buf.Bytes()
	checksum := crc32.ChecksumIEEE(data)

	// Write everything to the actual writer
	if _, err := w.Write(data); err != nil {
		return err
	}

	// Write Checksum
	if err := binary.Write(w, binary.LittleEndian, checksum); err != nil {
		return err
	}

	return nil
}

// LoadSnapshot recreates an HNSW index from a binary snapshot.
func LoadSnapshot(r io.Reader) (*Index, error) {
	// Read entire content to verify checksum
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot: %w", err)
	}

	if len(data) < 4 { // minimum size for checksum
		return nil, fmt.Errorf("snapshot too small")
	}

	// Verify checksum
	contentLen := len(data) - 4
	content := data[:contentLen]
	expectedChecksum := binary.LittleEndian.Uint32(data[contentLen:])
	actualChecksum := crc32.ChecksumIEEE(content)

	if actualChecksum != expectedChecksum {
		return nil, fmt.Errorf("checksum mismatch: expected %08x, got %08x", expectedChecksum, actualChecksum)
	}

	buf := bytes.NewReader(content)

	// 1. Magic and Version
	magicBytes := make([]byte, 4)
	if _, err := io.ReadFull(buf, magicBytes); err != nil {
		return nil, fmt.Errorf("failed to read magic: %w", err)
	}
	if string(magicBytes) != magicHNSW {
		return nil, fmt.Errorf("invalid magic string: %s", string(magicBytes))
	}

	v, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}
	if v != version {
		return nil, fmt.Errorf("unsupported version: %d", v)
	}

	// 2. Header
	var header [6]uint32
	if err := binary.Read(buf, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	idx := NewIndex(int(header[0]), int(header[2]))
	idx.Mmax0 = int(header[1])
	idx.Ef = int(header[3])
	idx.currentMaxLayer = int(header[4])
	idx.entryPoint = header[5]

	// 3. Node count
	var nodeCount uint32
	if err := binary.Read(buf, binary.LittleEndian, &nodeCount); err != nil {
		return nil, fmt.Errorf("failed to read node count: %w", err)
	}

	idx.nodes = make([]*Node, nodeCount)

	// 4. Nodes
	for i := uint32(0); i < nodeCount; i++ {
		var id, maxLayer uint32
		if err := binary.Read(buf, binary.LittleEndian, &id); err != nil {
			return nil, fmt.Errorf("failed to read node ID: %w", err)
		}
		if err := binary.Read(buf, binary.LittleEndian, &maxLayer); err != nil {
			return nil, fmt.Errorf("failed to read max layer for node %d: %w", id, err)
		}

		// Vector
		var vecLen uint32
		if err := binary.Read(buf, binary.LittleEndian, &vecLen); err != nil {
			return nil, fmt.Errorf("failed to read vector length: %w", err)
		}
		vector := make([]float32, vecLen)
		if err := binary.Read(buf, binary.LittleEndian, &vector); err != nil {
			return nil, fmt.Errorf("failed to read vector data: %w", err)
		}

		// Metadata
		var metaLen uint32
		if err := binary.Read(buf, binary.LittleEndian, &metaLen); err != nil {
			return nil, fmt.Errorf("failed to read metadata length: %w", err)
		}
		metaBytes := make([]byte, metaLen)
		if _, err := io.ReadFull(buf, metaBytes); err != nil {
			return nil, fmt.Errorf("failed to read metadata: %w", err)
		}
		var meta map[string]string
		if err := json.Unmarshal(metaBytes, &meta); err != nil {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}

		node := NewNode(id, vector, meta, int(maxLayer))

		// Links
		for l := uint32(0); l <= maxLayer; l++ {
			var linkCount uint32
			if err := binary.Read(buf, binary.LittleEndian, &linkCount); err != nil {
				return nil, fmt.Errorf("failed to read link count: %w", err)
			}
			links := make([]uint32, linkCount)
			if err := binary.Read(buf, binary.LittleEndian, &links); err != nil {
				return nil, fmt.Errorf("failed to read link IDs: %w", err)
			}
			node.Links[l] = links
		}

		idx.nodes[id] = node
	}

	// 5. String mappings
	var revCount uint32
	if err := binary.Read(buf, binary.LittleEndian, &revCount); err != nil {
		return nil, fmt.Errorf("failed to read reverse map count: %w", err)
	}

	idx.reverseMap = make([]string, revCount)
	idx.idMap = make(map[string]uint32, revCount)

	for i := uint32(0); i < revCount; i++ {
		var keyLen uint32
		if err := binary.Read(buf, binary.LittleEndian, &keyLen); err != nil {
			return nil, fmt.Errorf("failed to read key length: %w", err)
		}
		keyBytes := make([]byte, keyLen)
		if _, err := io.ReadFull(buf, keyBytes); err != nil {
			return nil, fmt.Errorf("failed to read key data: %w", err)
		}
		key := string(keyBytes)
		idx.reverseMap[i] = key
		idx.idMap[key] = i
	}

	idx.insertCount.Store(int64(nodeCount))
	return idx, nil
}
