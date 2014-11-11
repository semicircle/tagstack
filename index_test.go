package tagstack

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"testing"
	"time"
)

type testItem struct {
	id        uint64
	score     float64
	tags      []string
	tagscores []float64
	whoseid   uint64
}

func (item *testItem) Id() uint64 {
	return item.id
}

func (item *testItem) Score() (score float64) {
	return item.score
}

func (item *testItem) TagsWithScore() (tags []string, scores []float64) {
	return item.tags, item.tagscores
}

func (item *testItem) CreateDate() uint64 {
	return 100
}

func (item *testItem) WhoseId() uint64 {
	return item.whoseid
}

var (
	idx = &Index{
		What:                    "testing.index.",
		HighNodeBoundary:        3,
		Rule:                    dummyRule(),
		ItemLoadFunc:            itemLoadFunc,
		EnableRandomSuggestTags: true,
	}

	testvector = map[uint64]*testItem{
		1: &testItem{1, 1, []string{"A", "a1"}, []float64{1.0, 0.8}, 1},
		2: &testItem{2, 2, []string{"A", "a2"}, []float64{1.0, 0.8}, 2},
		3: &testItem{3, 3, []string{"A", "a3"}, []float64{1.0, 0.8}, 3},

		4: &testItem{4, 4, []string{"吃", "b1"}, []float64{1.0, 0.8}, 1},
		5: &testItem{5, 5, []string{"小吃", "b2"}, []float64{1.0, 0.8}, 2},
		6: &testItem{6, 6, []string{"骑行", "b3"}, []float64{1.0, 0.8}, 3},

		7: &testItem{7, 7, []string{"B", "A", "ab1"}, []float64{1.0, 0.8, 0.7}, 1},
		8: &testItem{8, 8, []string{"B", "A", "ab1"}, []float64{1.0, 0.8, 0.7}, 2},
		9: &testItem{9, 9, []string{"B", "A", "ab1"}, []float64{1.0, 0.8, 0.7}, 3},

		10: &testItem{10, 10, []string{"B", "A", "C", "abc1"}, []float64{1.0, 0.8, 0.6, 0.1}, 1},
		11: &testItem{11, 11, []string{"B", "A", "C", "abc2"}, []float64{1.0, 0.8, 0.6, 0.1}, 2},
		12: &testItem{12, 12, []string{"B", "A", "C", "abc3"}, []float64{1.0, 0.8, 0.6, 0.1}, 3},
	}
)

func itemLoadFunc(itemid uint64) Item {
	return testvector[itemid]
}

func getTestConn(shard int) redis.Conn {
	c, _ := redis.Dial("tcp", ":6379")
	return c
}

func initTest(num int) {
	c := getTestConn(0)
	c.Do("FLUSHDB")
	RedisShardMax = 1
	GetReadConn = getTestConn
	GetWriteConn = getTestConn
	idx.Init()
	for i := 1; i <= num; i++ {
		idx.Update(uint64(i))
	}
	time.Sleep(time.Millisecond * 20)
	idx.WaitAllIndexingDone()
}

func TestIndex1(t *testing.T) {
	initTest(1)

	fmt.Println("Search result 1:", idx.Query([]string{"A"}, 0, 9))
}

func TestIndex2(t *testing.T) {
	initTest(4)

	fmt.Println("Search result 2 :", idx.Query([]string{"好吃"}, 0, 9))
}

func TestIndex3(t *testing.T) {
	initTest(5)

	fmt.Println("Search result 3:", idx.Query([]string{"好吃"}, 0, 9))
}

func TestIndex4(t *testing.T) {
	initTest(6)

	fmt.Println("Search result 4:", idx.Query([]string{"自行车"}, 0, 9))
}

func TestIndex5(t *testing.T) {
	initTest(12)

	fmt.Println("Search result 5.1:", idx.Query([]string{"A", "B"}, 0, 9))
	fmt.Println("Search result 5.2:", idx.Query([]string{}, 0, 15))
}
