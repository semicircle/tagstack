// A tag indexing & searching system.
package tagstack

import (
	"github.com/garyburd/redigo/redis"
	"log"
	"os"
)

// Redis backend configuration:
type GetRedisConnFuncType func(shard_key int) redis.Conn

var (
	// To get reading / writing connections, tagstack is based on Redis & redigo.
	GetReadConn, GetWriteConn GetRedisConnFuncType

	RedisShardMax int
)

// Loggers.
var (
	// Normal logger.
	Logger = log.New(os.Stdout, "[tagstack]", log.LstdFlags)
	// Debug logger.
	DebugLogger = log.New(os.Stdout, "[tagstack.debug]", log.LstdFlags)
)

// What you should feed into this tag system.
type Item interface {
	// The item's identifier in uint64.
	Id() uint64

	// The basic score / value of this item.
	Score() float64

	// What's the item's tag, and the scores.
	// If there's no score among the tags, just pass scores with nil.
	TagsWithScore() (tags []string, scores []float64)

	// Whose item? the item id in uint64
	WhoseId() uint64

	// When did this item created.
	CreateDate() uint64
}

// The function type the system load a item. (thread-safe)
type ItemLoadFuncType func(id uint64) Item
