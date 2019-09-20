package sharding

import "hash/crc64"

// ShardFunc is a sharding function
type ShardFunc func(interface{}) int

// NewIntSharder shards on integers using a simple mod operation. The returned
// integer is the shard.
func NewIntSharder(max int64) ShardFunc {
	return func(val interface{}) int {
		switch v := val.(type) {
		case int:
			return int(v % int(max))
		case uint:
			return int(v % uint(max))
		case int32:
			return int(v % int32(max))
		case uint32:
			return int(v % uint32(max))
		case int64:
			return int(v % max)
		case uint64:
			return int(v % uint64(max))
		default:
			return 0
		}
	}
}

var crc64table = crc64.MakeTable(crc64.ISO)

// NewStringSharder hashes a string and calculates a shard based on the hash value. The returned
// integer is the shard id.
func NewStringSharder(max int) ShardFunc {
	return func(val interface{}) int {
		switch v := val.(type) {
		case string:
			return int(crc64.Checksum([]byte(v), crc64table) % uint64(max))
		case []byte:
			return int(crc64.Checksum(v, crc64table) % uint64(max))
		default:
			return 0
		}
	}
}
