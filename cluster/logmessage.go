package cluster

import "errors"

// LogMessageType is the log message type we're writing at the start of every log message
type LogMessageType byte

const (
	// ProposedShardMap is the new shard map the leader proposes (dictatorship-style)
	// to the followers. The payload for this message is a marshalled ShardManager
	// instance.
	ProposedShardMap LogMessageType = 1
	// ShardMapCommitted is a notification-only message that will synchronize the
	// shard map distributed in the previous message.
	ShardMapCommitted LogMessageType = 2
)

// LogMessage is log messages sent by the leader. There's only two types at this time.
// The payload is a byte array that can be unmarshalled into another type of message.
type LogMessage struct {
	MessageType LogMessageType
	Data        []byte
}

// NewLogMessage creates a new LogMessage instance
func NewLogMessage(t LogMessageType, data []byte) LogMessage {
	return LogMessage{
		MessageType: t,
		Data:        data,
	}
}

// MarshalBinary converts a Raft log message into a LogMessage struct.
func (m *LogMessage) MarshalBinary() ([]byte, error) {
	ret := make([]byte, 1)
	ret[0] = byte(m.MessageType)
	ret = append(ret, m.Data...)
	return ret, nil
}

// UnmarshalBinary unmarshals the byte array into this instance
func (m *LogMessage) UnmarshalBinary(buf []byte) error {
	if buf == nil || len(buf) < 1 {
		return errors.New("buffer is too short to unmarshal")
	}
	m.MessageType = LogMessageType(buf[0])
	m.Data = make([]byte, 0)
	m.Data = append(m.Data, buf[1:]...)
	return nil
}
