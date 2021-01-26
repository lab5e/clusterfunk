package funk

import (
	"encoding/binary"
	"errors"
)

// LogMessageType is the log message type we're writing at the start of every log message
type LogMessageType byte

const (
	// ProposedShardMap is the new shard map the leader proposes (dictatorship-style)
	// to the followers. The payload for this message is a marshalled ShardManager
	// instance.
	ProposedShardMap LogMessageType = 1

	// ShardMapCommitted is a  message that will synchronize the
	// shard map distributed in the previous message.
	ShardMapCommitted LogMessageType = 2

	// Bootstrap is a message that the leader appends to the log when the
	// cluster is first bootstrapped. The message holds the (local) time stamp
	// for the leader and indicates that the cluster was restarted. A replicated
	// log might contain more than one ClusterBootstrap message.
	Bootstrap LogMessageType = 3
)

// LogMessage is log messages sent by the leader.
// The payload is a byte array that can be unmarshalled into another type of message.
type LogMessage struct {
	MessageType LogMessageType
	AckEndpoint string
	Index       uint64
	Data        []byte
}

// NewLogMessage creates a new LogMessage instance
func NewLogMessage(t LogMessageType, endpoint string, data []byte) LogMessage {
	return LogMessage{
		MessageType: t,
		AckEndpoint: endpoint,
		Index:       0,
		Data:        data,
	}
}

// MarshalBinary converts a Raft log message into a LogMessage struct.
func (m *LogMessage) MarshalBinary() ([]byte, error) {
	ret := make([]byte, 2)
	ret[0] = byte(m.MessageType)
	ret[1] = byte(len(m.AckEndpoint))
	ret = append(ret, []byte(m.AckEndpoint)...)
	ret = append(ret, m.Data...)
	return ret, nil
}

// UnmarshalBinary unmarshals the byte array into this instance
func (m *LogMessage) UnmarshalBinary(buf []byte) error {
	if buf == nil || len(buf) < 3 {
		return errors.New("buffer is too short to unmarshal")
	}
	m.MessageType = LogMessageType(buf[0])
	strLen := buf[1]
	m.AckEndpoint = string(buf[2 : 2+strLen])
	m.Data = make([]byte, 0)
	m.Data = append(m.Data, buf[2+strLen:]...)
	return nil
}

// NewBootstrapMessage creates a new cluster bootstrap message
func NewBootstrapMessage(time int64) LogMessage {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(time))
	return LogMessage{
		MessageType: Bootstrap,
		AckEndpoint: "",
		Index:       0,
		Data:        buf,
	}
}

// GetBootstrapTime reads the cluster bootstrap time from the log message. If the
// message can't be decoded it will return -1
func GetBootstrapTime(m LogMessage) int64 {
	if m.MessageType != Bootstrap {
		return -1
	}
	return int64(binary.LittleEndian.Uint64(m.Data))
}
