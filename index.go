package tagstack

import (
	"github.com/garyburd/redigo/redis"
	"github.com/semicircle/gozhszht"
	"hash/adler32"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// the index struct
type Index struct {
	// To describe what's this index about, for example: Blog / News
	// And the string should be as short as possible, for the string is the prefix of all the keys in the redis database.
	What string

	// High Node Boundary:
	// A parameter to balance between memory usage & search speed:
	// the higher: the index size smaller & search slower.
	// normally a value of 100 is Ok.
	HighNodeBoundary int

	// the rule of this index.
	// please see type Rule struct for detail.
	Rule *Rule

	// Item loading funcation
	ItemLoadFunc ItemLoadFuncType

	// Optional: Enable this if you use 'RandomSuggestTags' method.
	// Note: If the items usually have more than 20 tags, this SHOULD NOT be enabled, because this feature will slow down the indexing progress to a "minutes per update" level.
	EnableRandomSuggestTags bool

	// private:
	initOnce    sync.Once
	initialized bool
	// the updating / removing job.
	chOp chan *job
	// wait if everything done.
	wgDone *sync.WaitGroup
	// rule
	rule *rule
}

// options.
type SORT_BY int

const (
	SORT_BY_SCORE = iota
	SORT_BY_DATE
	SORT_BY_OVERALL
)

type IndexOptions struct {
	SortBy   SORT_BY
	Reversal bool   // TODO: feature
	WhoseId  uint64 // TODO: feature
}

type job struct {
	id       uint64 // identifier of the item
	removing bool   // removing of the item
}

// This should be called once the struct is configured properly.
func (index *Index) Init() {
	index.initOnce.Do(func() {
		if index.HighNodeBoundary < 3 {
			Logger.Panicln("HighNodeBoundary < 3.")
		}

		if index.ItemLoadFunc == nil {
			Logger.Panicln("ItemLoadFunc is nil")
		}

		index.chOp = make(chan *job, index.HighNodeBoundary*50)
		index.wgDone = &sync.WaitGroup{}
		if index.Rule != nil {
			index.rule = index.Rule.init()
		}

		go index.workingRountine()

		index.initialized = true
	})
}

// Update an item:
// Note: If you are looking for: Create or New, use this instead.
// To reduce code complexity: we mix 2 kinds of request together:
// 1. Adding / Removing Tags
// 2. Update an item's score to affect the rank in searching.
// Update function will first remove old tags then refresh everything about the item together.
func (index *Index) Update(id uint64) {
	index.wgDone.Add(1)
	index.chOp <- &job{id: id}
	return
}

// Remove an item completely from the index.
func (index *Index) Remove(id uint64) {
	index.wgDone.Add(1)
	index.chOp <- &job{id: id, removing: true}
	return
}

// Query by tags in the [start, stop] range (including both start/stop)
func (index *Index) Query(tags []string, start, stop int) (ids []uint64) {
	return index.QueryOptions(tags, start, stop, &IndexOptions{SortBy: SORT_BY_OVERALL})
}

func (index *Index) QueryOptions(tags []string, start, stop int, options *IndexOptions) (ids []uint64) {
	var key string
	switch options.SortBy {
	case SORT_BY_SCORE:
		key = const_key_idx_score_rank
		break
	case SORT_BY_DATE:
		key = const_key_idx_date_rank
		break
	case SORT_BY_OVERALL:
		key = const_key_idx_overall_rank
		break
	}

	tags = convertTags2Simple(tags)
	tags = index.rule.applyRulesForSearching(tags)

	/* lucky ? */
	node := newIndexNode(index.What, tags, 1.0)
	if node.exists() {
		ids = node.itemsRevrange(key, start, stop)
	} else {
		/* not lucky: downgrade */
		// TODO: a quck and dirty implementation now, to get better, eg:
		// for (A,B,C), if node (ABC) doesn't exist, this should be downgrade as (AB,C)

		var min_node *index_node
		min_node_count := 100000
		for _, tag := range tags {
			node := newIndexNode(index.What, []string{tag}, 1.0)
			if cnt := node.itemCount(); cnt < min_node_count {
				min_node_count = cnt
				min_node = node
			}
		}

		ids = min_node.itemsRevrange(key, start, stop)
		for _, tag := range tags {
			if tag == min_node.tags[0] {
				continue
			}
			node := newIndexNode(index.What, []string{tag}, 1.0)
			ids = node.itemFilter(ids)
			if len(ids) == 0 {
				break
			}
		}
		return
	}

	return
}

// How many items have all the tags.
func (index *Index) ItemCount(tags []string) int {
	tags = convertTags2Simple(tags)
	tags = index.rule.applyRulesForSearching(tags)
	node := newIndexNode(index.What, tags, 1.0)
	return node.itemCount()
}

// What's the most frequently used tags with the tags ?
// Blame my poor language, in another way:
// Suggest a group of tags depends on a given group of tags.
func (index *Index) RelativeTags(tags []string, count int) (relative_tags []string) {
	tags = convertTags2Simple(tags)
	tags = index.rule.applyRulesForSearching(tags)
	node := newIndexNode(index.What, tags, 1.0)
	return node.relativeTags(count)
}

// Number of "relative tags".
func (index *Index) RelativeTagsCount(tags []string) int {
	return 0
}

func (index *Index) RelativeTagsOptions(tags []string, count int, options *IndexOptions) (relative_tags []string) {
	return index.RelativeTags(tags, count)
}

// Suggest some tag that
func (index *Index) RandomSuggestTags(tags []string, count int) (sugs []string) {
	tags = convertTags2Simple(tags)
	tags = index.rule.applyRulesForSearching(tags)
	node := newIndexNode(index.What, tags, 1.0)
	return node.randomSuggestTags(count)
}

// When you wonder if all the indexing jobs are all done
func (index *Index) WaitAllIndexingDone() {
	index.wgDone.Wait()
}

// job dispatcher:
func (index *Index) workingRountine() {
	defer func() {
		if x := recover(); x != nil {
			Logger.Panicln("index working routine panic:", x, string(debug.Stack()))
		}
	}()

	var op *job

	jobsMap := make(map[uint64]*job)

	for {
		select {
		case op = <-index.chOp:
			if op_old, ok := jobsMap[op.id]; ok && op.removing == op_old.removing {
				index.wgDone.Done()
			} else {
				jobsMap[op.id] = op
			}
			continue
		case <-time.After(time.Millisecond * 10):
			if len(jobsMap) != 0 {
				for _, op = range jobsMap {
					index.doIndxJob(op)
					index.wgDone.Done()
				}
				jobsMap = nil
				jobsMap = make(map[uint64]*job)
			} else {
				op = <-index.chOp
				jobsMap[op.id] = op
			}
		}
	}
}

// do the index / search jobs.
const (
	// tags separator
	const_tags_separator = "|"

	// item tag records
	const_key_item_tag_hash = "tith."

	// base set
	const_key_idx_base_set = "tbin."

	// high set
	const_key_high_tags_set = "thts."

	// sorting keys
	const_key_idx_score_rank   = "tsin."
	const_key_idx_date_rank    = "tdin."
	const_key_idx_overall_rank = "toin."

	// keys for relative tags feature
	const_key_idx_relative_rank = "trin."
	// keys for random suggest feature
	const_key_idx_rand_sug_set = "trss."
)

func (idx *Index) doIndxJob(op *job) {
	if !op.removing {
		idx.doUpdateJob(op)
	} else {
		idx.doRemoveJob(op)
	}
}

func (idx *Index) doRemoveJob(op *job) {
	DebugLogger.Println("doRemoveJob: ", idx.What, op.id)
	item := idx.ItemLoadFunc(op.id)
	last_taginfos := idx.itemTagInfos(op.id)

	for _, taginfo := range last_taginfos {
		n := newIndexNode(idx.What, []string{taginfo.title}, 1.0)
		n.detach(item)
		n.detach_deeper(item)
		for _, alias := range taginfo.aliases {
			n := newIndexNode(idx.What, []string{alias}, 1.0)
			n.detach(item)
			n.detach_deeper(item)
		}
	}
}

func (idx *Index) doUpdateJob(op *job) {
	DebugLogger.Println("doUpdateJob: ", idx.What, op.id)
	item := idx.ItemLoadFunc(op.id)

	// load the current tags.
	curr_tags, scores := item.TagsWithScore()

	// apply rules - prepare
	curr_taginfos := make([]*taginfo, len(curr_tags), len(curr_tags)+1)
	for i, tag := range curr_tags {
		curr_taginfos[i] = &taginfo{}
		curr_taginfos[i].title = tag
		curr_taginfos[i].score = scores[i]
		curr_taginfos[i].enrelative = true
	}
	// apply rules - whose
	if whose_id := item.WhoseId(); whose_id != 0 {
		curr_taginfos = append(curr_taginfos, &taginfo{title: "belongs_to:" + strconv.Itoa(int(whose_id)), score: float64(1.0), enrelative: false})
	}
	// apply rules - fire
	curr_taginfos = idx.rule.applyRulesForIndexing(curr_taginfos)

	// dbgstr := make([]string, len(curr_taginfos))
	// for i, info := range curr_taginfos {
	// 	dbgstr[i] = info.title
	// }
	// DebugLogger.Println("doUpdateJob dbgstr:", dbgstr)

	// load item tags
	last_tags := idx.itemTagInfos(op.id)

	if len(last_tags) != 0 {
		// DebugLogger.Println("doUpdateJob last_tags:", last_tags, "curr_tags", curr_taginfos)

		// figure out which to remove.
		removing_tags := make([]*taginfo, 0, 10)

		sort.Sort(taginfo_title_sorter(last_tags))
		sort.Sort(taginfo_title_sorter(curr_taginfos))
		last_i, curr_i := 0, 0
		for {
			if curr_i == len(curr_taginfos) {
				removing_tags = append(removing_tags, last_tags[last_i:]...)
				break
			}

			if last_i == len(last_tags) {
				break
			}

			// compare
			last_sel := last_tags[last_i]
			curr_sel := curr_taginfos[curr_i]

			if last_sel.title == curr_sel.title {
				last_i++
				curr_i++
				continue
			}

			if last_sel.title < curr_sel.title {
				removing_tags = append(removing_tags, last_sel)
				last_i++
				continue
			}

			if last_sel.title > curr_sel.title {
				curr_i++
				continue
			}
		}

		DebugLogger.Println("doUpdateJob removing_tags:", removing_tags)

		// removing
		for _, taginfo := range removing_tags {
			// 1. remove basically ?
			n := newIndexNode(idx.What, []string{taginfo.title}, 1.0)
			n.detach(item)

			// 2. is there highnodes ?
			n.detach_deeper(item)

			// 3. aliases.
			for _, alias := range taginfo.aliases {
				n := newIndexNode(idx.What, []string{alias}, 1.0)
				n.detach(item)
				n.detach_deeper(item)
			}
		}

	}

	// fill item tags:
	idx.setItemTagInfos(op.id, curr_taginfos)

	// updating

	// 1. update basic nodes & pick high ones out
	high_tags_mat := make([][]string, 0, len(curr_taginfos))
	high_scores_mat := make([][]float64, 0, len(curr_taginfos))
	en_relative_vector := make([]bool, 0, len(curr_taginfos))
	vector_count := 1
	expd_div := make([]int, 0, len(curr_taginfos))
	expd_mod := make([]int, 0, len(curr_taginfos))
	for _, taginfo := range curr_taginfos {
		high_tags := make([]string, 0, 10)
		high_scores := make([]float64, 0, 10)

		n := newIndexNode(idx.What, []string{taginfo.title}, 1.0)

		n.attach(item)

		if idx.updatingBombTest(n) {
			return
		}

		if n.isHigh() {
			high_tags = append(high_tags, taginfo.title)
			high_scores = append(high_scores, 1.0)
		}

		for i, alias := range taginfo.aliases {
			n := newIndexNode(idx.What, []string{alias}, taginfo.alias_scores[i])
			n.attach(item)

			if idx.updatingBombTest(n) {
				return
			}

			if n.isHigh() {
				high_tags = append(high_tags, alias)
				high_scores = append(high_scores, taginfo.alias_scores[i])
			}
		}

		if len(high_tags) != 0 {
			high_tags_mat = append(high_tags_mat, high_tags)
			high_scores_mat = append(high_scores_mat, high_scores)
			en_relative_vector = append(en_relative_vector, taginfo.enrelative)
			expd_div = append(expd_div, vector_count)
			expd_mod = append(expd_mod, len(high_tags))
			vector_count = vector_count * len(high_tags)
		}
	}

	// 2. high nodes game.
	if len(high_tags_mat) >= 2 {
		for i := 0; i < vector_count; i++ {
			tags_vector := make([]string, len(high_tags_mat))
			scores_vector := make([]float64, len(high_tags_mat))
			for j := 0; j < len(high_tags_mat); j++ {
				s := i / expd_div[j] % expd_mod[j]
				tags_vector[j] = high_tags_mat[j][s]
				scores_vector[j] = high_scores_mat[j][s]
			}
			s := &updateSorter{tags: tags_vector, scores: scores_vector, en_relative_vector: en_relative_vector}
			sort.Sort(s)
			n := newIndexNode(idx.What, nil, 1.0)
			idx.updatingDeeper(n, true, s.tags, s.scores, s.en_relative_vector, item)

			// 3. Random Suggestion.
			if i == 0 && idx.EnableRandomSuggestTags {
				leng := len(tags_vector)
				if leng > 10 {
					leng = 10
				}
				for i := 0; i < leng; i++ {
					nl1 := newIndexNode(idx.What, []string{tags_vector[i]}, 1.0)
					nl1s := make([]string, 0, leng-1)
					for j := 0; j < i; j++ {
						if i != j {
							nl1s = append(nl1s, tags_vector[j])

							// nl2 := newIndexNode(idx.What, []string{tags_vector[i], tags_vector[j]}, 1.0)
							// nl2s := make([]string, 0, leng-2)
							// for k := 0; k < leng; k++ {
							// 	if i != k && j != k {
							// 		nl2s = append(nl2s, tags_vector[k])
							// 	}
							// }
							// if len(nl2s) != 0 {
							// 	nl2.addRandomSuggestTags(nl2s)
							// }
						}
					}
					if len(nl1s) != 0 {
						nl1.addRandomSuggestTags(nl1s)
					}
				}
			}
		}
	}

}

type updateSorter struct {
	tags               []string
	scores             []float64
	en_relative_vector []bool
}

func (s *updateSorter) Len() int { return len(s.tags) }
func (s *updateSorter) Swap(i, j int) {
	s.tags[i], s.tags[j] = s.tags[j], s.tags[i]
	s.scores[i], s.scores[j] = s.scores[j], s.scores[i]
	s.en_relative_vector[i], s.en_relative_vector[j] = s.en_relative_vector[j], s.en_relative_vector[i]
}
func (s *updateSorter) Less(i, j int) bool { return s.tags[i] < s.tags[j] }

func (idx *Index) updatingDeeper(n *index_node, en_relative bool, right_tags []string, right_score []float64, en_relative_vector []bool, item Item) {
	// DebugLogger.Println("updatingDeeper:", right_tags)
	var itemcount int

	n.attach(item)

	if en_relative {
		itemcount = n.itemCount()
	}

	if idx.updatingBombTest(n) {
		return
	}

	// develop deeper
	if n.isHigh() {
		len_right := len(right_tags)
		if len_right != 0 {
			for i := 0; i < len_right; i++ {
				next := newIndexNode(idx.What, append(n.tags, right_tags[i]), n.tags_score*right_score[i])
				idx.updatingDeeper(next, en_relative && en_relative_vector[i], right_tags[i+1:], right_score[i+1:], en_relative_vector[i+1:], item)
			}
		}
	}

	if en_relative {
		lentags := len(n.tags)
		if lentags >= 2 {
			curr := make([]string, lentags-1)
			copy(curr, n.tags[1:])
			for i := 0; i < lentags; i++ {
				// DebugLogger.Println("setRelativeTags:", i, curr, n.tags[i], n.tags)
				nr := newIndexNode(idx.What, curr, 1.0)
				nr.setRelativeTags(n.tags[i], itemcount)
				if i != lentags-1 {
					curr[i] = n.tags[i]
				}
			}
		}
	}

}

func (idx *Index) updatingBombTest(n *index_node) bool {
	if n.itemCount() == idx.HighNodeBoundary && !n.isHigh() {
		n.setHigh()
		ids := n.items()
		idx.wgDone.Add(len(ids))
		Logger.Println("A new high tag:", idx.What, n.tags)
		Logger.Println("Affected items:", idx.What, "ids:", ids)
		go func() {
			for _, id := range ids {
				idx.chOp <- &job{id: id}
			}
		}()
		return true
	}
	return false
}

func (idx *Index) itemTagInfos(id uint64) []*taginfo {
	key := const_key_item_tag_hash + strconv.Itoa(int(id))
	c := GetReadConn(int(id))
	defer c.Close()
	vals, _ := redis.Values(c.Do("HGETALL", key))
	fields := make([]string, len(vals))
	ast(redis.ScanSlice(vals, &fields))
	ret := make([]*taginfo, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		x := &taginfo{}
		x.title = fields[i]
		x.aliases = strings.Split(fields[i+1], const_tags_separator)
		ret[i/2] = x
	}
	return ret
}

func (idx *Index) setItemTagInfos(id uint64, infos []*taginfo) {
	c := GetWriteConn(int(id))
	defer c.Close()

	key := const_key_item_tag_hash + strconv.Itoa(int(id))

	c.Send("DEL", key)
	for _, info := range infos {
		ast(c.Send("HSET", key, info.title, strings.Join(info.aliases, const_tags_separator)))
	}
	ast(c.Flush())
	ast2(c.Receive())
	for i := 0; i < len(infos); i++ {
		ast2(c.Receive())
	}

}

func (idx *Index) node_str(key string, tags []string) string {
	return idx.What + key + strings.Join(tags, const_tags_separator)
}

// info of a tag index node.
type index_node struct {
	what       string
	tags_score float64
	tags       []string

	// cache fields
	node  string
	exist *bool
	shard int
}

func newIndexNode(what string, tags []string, tags_score float64) (node *index_node) {
	node = &index_node{what: what, tags: tags, tags_score: tags_score}
	if node.node == "" {
		sort.Strings(tags)
		node.node = strings.Join(tags, const_tags_separator)
		node.shard = int(adler32.Checksum([]byte(node.node))) //TODO:
	}
	return node
}

func (node *index_node) idstr(str string) string {
	return node.what + str + node.node
}

func (node *index_node) attach(item Item) {
	c := GetWriteConn(node.shard)
	defer c.Close()

	// variables
	item_id := item.Id()
	item_score := item.Score()
	item_date := item.CreateDate()

	// base set
	c.Send("SADD", node.idstr(const_key_idx_base_set), item_id)

	// pure score ascend index.
	c.Send("ZADD", node.idstr(const_key_idx_score_rank), item_score, item_id)

	// pure date ascend index.
	c.Send("ZADD", node.idstr(const_key_idx_date_rank), item_date, item_id)

	// overall score.
	c.Send("ZADD", node.idstr(const_key_idx_overall_rank), fade_score(item_score*node.tags_score, item_date), item_id)

	c.Flush()
	ast2(c.Receive())
	ast2(c.Receive())
	ast2(c.Receive())
	ast2(c.Receive())
}

func (node *index_node) detach(item Item) {
	c := GetWriteConn(node.shard)
	defer c.Close()

	item_id := item.Id()

	c.Send("SREM", node.idstr(const_key_idx_base_set), item_id)
	c.Send("ZREM", node.idstr(const_key_idx_score_rank), item_id)
	c.Send("ZREM", node.idstr(const_key_idx_date_rank), item_id)
	c.Send("ZREM", node.idstr(const_key_idx_overall_rank), item_id)
	c.Flush()
	ast2(c.Receive())
	ast2(c.Receive())
	ast2(c.Receive())
	ast2(c.Receive())
}

func (node *index_node) detach_deeper(item Item) {
	// find nodes and kill the all.
	item_id := item.Id()
	wg := &sync.WaitGroup{}
	wg.Add(RedisShardMax)
	for i := 0; i < RedisShardMax; i++ {
		go func(shard int) {
			c := GetWriteConn(shard)
			defer c.Close()
			pattern := "*"
			pattern += strings.Join(node.tags, "*")
			pattern += "*"

			cursor := "0"
			nodes := make([]string, 0, 10)
			for {
				vals, _ := redis.Values(ast2(c.Do("SSCAN", const_key_high_tags_set+node.what, cursor, "MATCH", pattern)))
				cursor = string(vals[0].([]byte))
				for _, ikey := range vals[1].([]interface{}) {
					nodes = append(nodes, string(ikey.([]byte)))
				}
				if cursor == "0" {
					break
				}
			}

			for _, n := range nodes {
				c.Send("SREM", node.what+const_key_idx_base_set+n, item_id)
				c.Send("ZREM", node.what+const_key_idx_score_rank+n, item_id)
				c.Send("ZREM", node.what+const_key_idx_date_rank+n, item_id)
				c.Send("ZREM", node.what+const_key_idx_overall_rank+n, item_id)
			}
			c.Flush()
			for i := 0; i < len(nodes); i++ {
				ast2(c.Receive())
				ast2(c.Receive())
				ast2(c.Receive())
				ast2(c.Receive())
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func (node *index_node) exists() bool {
	if node.exist == nil {
		c := GetReadConn(node.shard)
		defer c.Close()
		exist, _ := redis.Bool(ast2(c.Do("EXISTS", node.idstr(const_key_idx_base_set))))
		node.exist = &exist
	}
	return *node.exist
}

func (node *index_node) itemCount() int {
	c := GetReadConn(node.shard)
	defer c.Close()
	count, _ := redis.Int(ast2(c.Do("SCARD", node.idstr(const_key_idx_base_set))))
	return count
}

func (node *index_node) items() (ids []uint64) {
	if !node.exists() {
		return nil
	}
	c := GetReadConn(node.shard)
	defer c.Close()
	vals, _ := redis.Values(c.Do("SMEMBERS", node.idstr(const_key_idx_base_set)))
	ids = make([]uint64, len(vals))
	ast(redis.ScanSlice(vals, &ids))
	return
}

func (node *index_node) itemFilter(subjects []uint64) (confirmed_ids []uint64) {
	c := GetReadConn(node.shard)
	defer c.Close()
	key := node.idstr(const_key_idx_base_set)
	for _, id := range subjects {
		c.Send("SISMEMBER", key, id)
	}
	c.Flush()
	for _, id := range subjects {
		if exists, _ := redis.Bool(ast2(c.Receive())); exists {
			confirmed_ids = append(confirmed_ids, id)
		}
	}
	return
}

func (node *index_node) itemsRange(sorting_key string, start, stop int) (ids []uint64) {
	return node.itemsWith("ZRANGE", sorting_key, start, stop)
}

func (node *index_node) itemsRevrange(sorting_key string, start, stop int) (ids []uint64) {
	return node.itemsWith("ZREVRANGE", sorting_key, start, stop)
}

func (node *index_node) itemsWith(cmd, sorting_key string, start, stop int) (ids []uint64) {
	c := GetReadConn(node.shard)
	defer c.Close()
	key := node.idstr(sorting_key)
	vals, _ := redis.Values(ast2(c.Do(cmd, key, start, stop)))
	ids = make([]uint64, len(vals))
	ast(redis.ScanSlice(vals, &ids))
	return
}

func (node *index_node) setRelativeTags(tag string, times int) {
	// if strings.HasPrefix(tag, "belongs_to") {
	// 	return
	// }

	// DebugLogger.Println("setRelativeTags:", node.tags, "to:", tag, "times", times)
	c := GetWriteConn(node.shard)
	defer c.Close()
	ast2(c.Do("ZADD", node.idstr(const_key_idx_relative_rank), times, tag))
}

func (node *index_node) relativeTags(count int) []string {
	c := GetReadConn(node.shard)
	defer c.Close()
	rels, _ := redis.Strings(ast2(c.Do("ZREVRANGE", node.idstr(const_key_idx_relative_rank), 0, count-1)))
	return rels
}

func (node *index_node) addRandomSuggestTags(tags []string) {
	c := GetWriteConn(node.shard)
	defer c.Close()
	args := make([]interface{}, len(tags)+1)
	args[0] = interface{}(node.idstr(const_key_idx_rand_sug_set))
	for i, tag := range tags {
		args[i] = interface{}(tag)
	}
	ast2(c.Do("SADD", args...))
}

func (node *index_node) randomSuggestTags(count int) []string {
	c := GetReadConn(node.shard)
	defer c.Close()
	rels, _ := redis.Strings(ast2(c.Do("SRANDMEMBER", node.idstr(const_key_idx_rand_sug_set), count)))
	return rels
}

func (node *index_node) setHigh() {
	c := GetWriteConn(node.shard)
	defer c.Close()
	ast2(c.Do("SADD", const_key_high_tags_set+node.what, node.node))
}

func (node *index_node) isHigh() bool {
	c := GetReadConn(node.shard)
	defer c.Close()
	ret, _ := redis.Bool(ast2(c.Do("SISMEMBER", const_key_high_tags_set+node.what, node.node)))
	return ret
}

// some helper functions below for keeping the code short.
func str2shard(str string) int {
	return int(adler32.Checksum([]byte(str)))
}

func convertTags2Simple(tags []string) []string {
	for i, tag := range tags {
		tags[i] = gozhszht.ToSimple(tag)
	}
	return tags
}

func must(exp bool, what ...interface{}) {
	if exp == false {
		Logger.Panicln(what...)
	}
}

func mastnt(exp bool, what ...interface{}) {
	if exp == true {
		Logger.Panicln(what...)
	}
}

func ast(err error) {
	if err != nil {
		Logger.Panicln(err)
	}
}

func ast2(dontcare interface{}, err error) (interface{}, error) {
	if err != nil {
		Logger.Panicln(err)
	}
	return dontcare, nil
}
