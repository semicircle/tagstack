package tagstack

import (
	"github.com/garyburd/redigo/redis"
	"log"
	"os"
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
		3: &testItem{3, 3, []string{"A", "住宿"}, []float64{1.0, 0.8}, 3},

		4: &testItem{4, 4, []string{"吃", "b1"}, []float64{1.0, 0.8}, 1},
		5: &testItem{5, 5, []string{"小吃", "b2"}, []float64{1.0, 0.8}, 2},
		6: &testItem{6, 6, []string{"骑行", "b3"}, []float64{1.0, 0.8}, 3},

		7: &testItem{7, 7, []string{"B", "A", "ab1"}, []float64{1.0, 0.8, 0.7}, 1},
		8: &testItem{8, 8, []string{"B", "A", "ab1"}, []float64{1.0, 0.8, 0.7}, 2},
		9: &testItem{9, 9, []string{"B", "A", "住宿"}, []float64{1.0, 0.8, 0.7}, 3},

		10: &testItem{10, 10, []string{"B", "A", "C", "abc1"}, []float64{1.0, 0.8, 0.6, 0.1}, 1},
		11: &testItem{11, 11, []string{"B", "A", "C", "abc2"}, []float64{1.0, 0.8, 0.6, 0.1}, 2},
		12: &testItem{12, 12, []string{"B", "A", "C", "住宿"}, []float64{1.0, 0.8, 0.6, 0.1}, 3},

		13: &testItem{12, 12, []string{"红墨咖啡", "鼓浪屿", "推荐", "客栈", "住", "酒店", "杨桃院子"}, []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0}, 4},
	}
)

func itemLoadFunc(itemid uint64) Item {
	return testvector[itemid]
}

func getTestConn(shard int) redis.Conn {
	c, err := redis.Dial("tcp", "127.0.0.1:6379")
	// logger := log.New(os.Stdout, "[db]", log.LstdFlags)
	// c = redis.NewLoggingConn(c, logger, "[test.db]")
	if err != nil {
		time.Sleep(time.Second)
		c, err = redis.Dial("tcp", ":6379")
		if err != nil {
			log.Panicln("Test Connection Down?", err)
		}
	}
	return c
}

func initTest(num int) {
	DebugLogger = log.New(os.Stdout, "[tagstack.debug]", log.LstdFlags)
	c := getTestConn(0)
	c.Do("FLUSHDB")
	RedisShardMax = 1
	GetReadConn = getTestConn
	GetWriteConn = getTestConn
	idx.Init()
	for i := 1; i <= num; i++ {
		idx.Update(uint64(i))
	}
	// time.Sleep(time.Millisecond * 20)
	idx.WaitAllIndexingDone()
}

// Basic
func TestIndex1(t *testing.T) {
	initTest(1)
	ids := idx.Query([]string{"A"}, 0, 9)
	must(len(ids) == 1 && ids[0] == 1, "Search result:", ids)
}

// Normalize
func TestIndex2(t *testing.T) {
	initTest(4)
	ids := idx.Query([]string{"好吃"}, 0, 9)
	must(len(ids) == 1 && ids[0] == 4, "Search result:", ids)
}

// Normalize & Contain
func TestIndex3(t *testing.T) {
	initTest(5)
	ids := idx.Query([]string{"好吃"}, 0, 9)
	must(len(ids) == 2 && ids[0] == 5 && ids[1] == 4, "Search result:", ids)
}

// Entanglement
func TestIndex4(t *testing.T) {
	initTest(6)
	ids := idx.Query([]string{"自行车"}, 0, 9)
	must(len(ids) == 1 && ids[0] == 6, "Search result:", ids)
}

// Combination & High node
func TestIndex5(t *testing.T) {
	initTest(12)
	ids := idx.Query([]string{"A", "B"}, 0, 9)
	must(len(ids) == 6 && ids[0] == 12 && ids[5] == 7, "Search result:", ids)

	// fmt.Println("Search result 5.2:", idx.Query([]string{}, 0, 15))
}

// Tag removing
func TestIndex6(t *testing.T) {
	initTest(12)
	testvector[12].tags = []string{"B", "A", "abc3"}

	idx.Update(12)
	// time.Sleep(time.Millisecond * 20)
	idx.WaitAllIndexingDone()

	ids := idx.Query([]string{"A", "B"}, 0, 9)
	must(len(ids) == 6 && ids[0] == 12 && ids[5] == 7, "Search result:", ids)
	ids = idx.Query([]string{"A", "C"}, 0, 9)
	must(len(ids) == 2 && ids[0] == 11 && ids[1] == 10, "Search result:", ids)
	ids = idx.Query([]string{"C"}, 0, 9)
	must(len(ids) == 2 && ids[0] == 11 && ids[1] == 10, "Search result:", ids)
}

// Removing item
func TestIndex7(t *testing.T) {
	initTest(12)

	idx.Remove(12)
	// time.Sleep(time.Millisecond * 20)
	idx.WaitAllIndexingDone()

	ids := idx.Query([]string{"A", "B"}, 0, 9)
	must(len(ids) == 5 && ids[0] == 11 && ids[4] == 7, "Search result:", ids)
	ids = idx.Query([]string{"A", "C"}, 0, 9)
	must(len(ids) == 2 && ids[0] == 11 && ids[1] == 10, "Search result:", ids)
	ids = idx.Query([]string{"C"}, 0, 9)
	must(len(ids) == 2 && ids[0] == 11 && ids[1] == 10, "Search result:", ids)

}

// Bug
func TestIndex8(t *testing.T) {
	initTest(13)
}
