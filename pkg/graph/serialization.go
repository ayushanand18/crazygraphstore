// Package graph provides serialization for graph data structures.
package graph

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
)

// Serializer handles encoding and decoding of graph data structures.
type Serializer struct{}

// NewSerializer creates a new serializer.
func NewSerializer() *Serializer {
	return &Serializer{}
}

// SerializeNode encodes a node to bytes.
func (s *Serializer) SerializeNode(node *Node) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write ID
	if err := s.writeString(buf, node.ID); err != nil {
		return nil, fmt.Errorf("failed to write ID: %w", err)
	}

	// Write labels count and labels
	if err := binary.Write(buf, binary.BigEndian, uint32(len(node.Labels))); err != nil {
		return nil, err
	}
	for _, label := range node.Labels {
		if err := s.writeString(buf, label); err != nil {
			return nil, err
		}
	}

	// Write properties count
	if err := binary.Write(buf, binary.BigEndian, uint32(len(node.Properties))); err != nil {
		return nil, err
	}

	// Write properties
	for key, value := range node.Properties {
		if err := s.writeString(buf, key); err != nil {
			return nil, err
		}
		if err := s.writeValue(buf, value); err != nil {
			return nil, fmt.Errorf("failed to write property %s: %w", key, err)
		}
	}

	// Write version
	if err := binary.Write(buf, binary.BigEndian, node.Version); err != nil {
		return nil, err
	}

	// Write timestamps
	if err := binary.Write(buf, binary.BigEndian, node.CreatedAt.Unix()); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, node.UpdatedAt.Unix()); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DeserializeNode decodes a node from bytes.
func (s *Serializer) DeserializeNode(data []byte) (*Node, error) {
	buf := bytes.NewReader(data)
	node := &Node{
		Properties: make(map[string]interface{}),
	}

	// Read ID
	id, err := s.readString(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read ID: %w", err)
	}
	node.ID = id

	// Read labels
	var labelCount uint32
	if err := binary.Read(buf, binary.BigEndian, &labelCount); err != nil {
		return nil, err
	}
	node.Labels = make([]string, labelCount)
	for i := uint32(0); i < labelCount; i++ {
		label, err := s.readString(buf)
		if err != nil {
			return nil, err
		}
		node.Labels[i] = label
	}

	// Read properties
	var propCount uint32
	if err := binary.Read(buf, binary.BigEndian, &propCount); err != nil {
		return nil, err
	}
	for i := uint32(0); i < propCount; i++ {
		key, err := s.readString(buf)
		if err != nil {
			return nil, err
		}
		value, err := s.readValue(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read property %s: %w", key, err)
		}
		node.Properties[key] = value
	}

	// Read version
	if err := binary.Read(buf, binary.BigEndian, &node.Version); err != nil {
		return nil, err
	}

	// Read timestamps
	var createdAt, updatedAt int64
	if err := binary.Read(buf, binary.BigEndian, &createdAt); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &updatedAt); err != nil {
		return nil, err
	}
	node.CreatedAt = time.Unix(createdAt, 0)
	node.UpdatedAt = time.Unix(updatedAt, 0)

	return node, nil
}

// SerializeEdge encodes an edge to bytes.
func (s *Serializer) SerializeEdge(edge *Edge) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write ID
	if err := s.writeString(buf, edge.ID); err != nil {
		return nil, err
	}

	// Write node IDs
	if err := s.writeString(buf, edge.FromNodeID); err != nil {
		return nil, err
	}
	if err := s.writeString(buf, edge.ToNodeID); err != nil {
		return nil, err
	}

	// Write type
	if err := s.writeString(buf, edge.Type); err != nil {
		return nil, err
	}

	// Write properties count
	if err := binary.Write(buf, binary.BigEndian, uint32(len(edge.Properties))); err != nil {
		return nil, err
	}

	// Write properties
	for key, value := range edge.Properties {
		if err := s.writeString(buf, key); err != nil {
			return nil, err
		}
		if err := s.writeValue(buf, value); err != nil {
			return nil, fmt.Errorf("failed to write property %s: %w", key, err)
		}
	}

	// Write version
	if err := binary.Write(buf, binary.BigEndian, edge.Version); err != nil {
		return nil, err
	}

	// Write timestamps
	if err := binary.Write(buf, binary.BigEndian, edge.CreatedAt.Unix()); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, edge.UpdatedAt.Unix()); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DeserializeEdge decodes an edge from bytes.
func (s *Serializer) DeserializeEdge(data []byte) (*Edge, error) {
	buf := bytes.NewReader(data)
	edge := &Edge{
		Properties: make(map[string]interface{}),
	}

	// Read ID
	id, err := s.readString(buf)
	if err != nil {
		return nil, err
	}
	edge.ID = id

	// Read node IDs
	fromNodeID, err := s.readString(buf)
	if err != nil {
		return nil, err
	}
	edge.FromNodeID = fromNodeID

	toNodeID, err := s.readString(buf)
	if err != nil {
		return nil, err
	}
	edge.ToNodeID = toNodeID

	// Read type
	edgeType, err := s.readString(buf)
	if err != nil {
		return nil, err
	}
	edge.Type = edgeType

	// Read properties
	var propCount uint32
	if err := binary.Read(buf, binary.BigEndian, &propCount); err != nil {
		return nil, err
	}
	for i := uint32(0); i < propCount; i++ {
		key, err := s.readString(buf)
		if err != nil {
			return nil, err
		}
		value, err := s.readValue(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read property %s: %w", key, err)
		}
		edge.Properties[key] = value
	}

	// Read version
	if err := binary.Read(buf, binary.BigEndian, &edge.Version); err != nil {
		return nil, err
	}

	// Read timestamps
	var createdAt, updatedAt int64
	if err := binary.Read(buf, binary.BigEndian, &createdAt); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &updatedAt); err != nil {
		return nil, err
	}
	edge.CreatedAt = time.Unix(createdAt, 0)
	edge.UpdatedAt = time.Unix(updatedAt, 0)

	return edge, nil
}

// Helper methods for writing and reading basic types

func (s *Serializer) writeString(buf *bytes.Buffer, str string) error {
	if err := binary.Write(buf, binary.BigEndian, uint32(len(str))); err != nil {
		return err
	}
	_, err := buf.WriteString(str)
	return err
}

func (s *Serializer) readString(buf *bytes.Reader) (string, error) {
	var length uint32
	if err := binary.Read(buf, binary.BigEndian, &length); err != nil {
		return "", err
	}
	str := make([]byte, length)
	if _, err := buf.Read(str); err != nil {
		return "", err
	}
	return string(str), nil
}

// Property value types
const (
	typeString  byte = 1
	typeInt64   byte = 2
	typeFloat64 byte = 3
	typeBool    byte = 4
	typeBytes   byte = 5
)

func (s *Serializer) writeValue(buf *bytes.Buffer, value interface{}) error {
	switch v := value.(type) {
	case string:
		if err := buf.WriteByte(typeString); err != nil {
			return err
		}
		return s.writeString(buf, v)
	case int:
		if err := buf.WriteByte(typeInt64); err != nil {
			return err
		}
		return binary.Write(buf, binary.BigEndian, int64(v))
	case int64:
		if err := buf.WriteByte(typeInt64); err != nil {
			return err
		}
		return binary.Write(buf, binary.BigEndian, v)
	case float64:
		if err := buf.WriteByte(typeFloat64); err != nil {
			return err
		}
		return binary.Write(buf, binary.BigEndian, v)
	case bool:
		if err := buf.WriteByte(typeBool); err != nil {
			return err
		}
		var b byte
		if v {
			b = 1
		}
		return buf.WriteByte(b)
	case []byte:
		if err := buf.WriteByte(typeBytes); err != nil {
			return err
		}
		if err := binary.Write(buf, binary.BigEndian, uint32(len(v))); err != nil {
			return err
		}
		_, err := buf.Write(v)
		return err
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
}

func (s *Serializer) readValue(buf *bytes.Reader) (interface{}, error) {
	typeByte, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}

	switch typeByte {
	case typeString:
		return s.readString(buf)
	case typeInt64:
		var v int64
		err := binary.Read(buf, binary.BigEndian, &v)
		return v, err
	case typeFloat64:
		var v float64
		err := binary.Read(buf, binary.BigEndian, &v)
		return v, err
	case typeBool:
		b, err := buf.ReadByte()
		return b == 1, err
	case typeBytes:
		var length uint32
		if err := binary.Read(buf, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		data := make([]byte, length)
		if _, err := buf.Read(data); err != nil {
			return nil, err
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unknown type byte: %d", typeByte)
	}
}
