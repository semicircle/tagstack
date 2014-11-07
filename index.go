// A tag indexing & searching system.
package tagstack

import (
	"sync"
)

// what type of struct you should feed into this system?
type Item interface {
	// The item's identifier in uint64.
	Id() uint64

	// The basic score / value of this item.
	Score() int

	// What's the item's tag, and the scores.
	// If there's no score among the tags, just pass scores with nil.
	TagsWithScore() (tags []string, scores []int)

	// Whose item? the item id in uint64
	WhoseId() uint64

	// When did this item created.
	CreateDate() uint64
}

// the index struct
type TagIndex struct {
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
	ItemLoadFunc func(id uint64) Item

	// Optional: Enable this if you use 'RandomSuggestTags' method.
	// Note: If the items usually have more than 20 tags, this SHOULD NOT be enabled, because this feature will slow down the indexing progress to a "minutes per update" level.
	EnableRandomSuggestTags bool

	// private:
	initialized bool
	// the updating / removing job.
	chOp chan *job
	// wait if everything done.
	wgDone *sync.WaitGroup
	// rule
	rule *rule
}

type job struct {
	id       uint64 // identifier of the item
	removing bool   // removing of the item
}

// This should be called once the struct is configured properly.
func (index *TagIndex) Init() {

	if index.Rule != nil {
		index.rule = index.Rule.init()
	}

	index.initialized = true
}

// Update an item:
// Note: If you are looking for some method named: Create/New, use this.
func (index *TagIndex) Update(id uint64) {
	return
}

// Remove an item.
func (index *TagIndex) Remove(id uint64) {
	return
}

// Query by tags in the [start, stop] range (including both start/stop)
func (index *TagIndex) Query(tags []string, start, stop int) (id []uint64) {
	return nil
}

func (index *TagIndex) QueryOptions(tags []string, start, stop int, options *IndexOptions) (ids []uint64) {
	return nil
}

// How many items have all the tags.
func (index *TagIndex) ItemCount(tags []string) int {
	return 0
}

// What's the most frequently used tags with the tags ?
// Blame my poor language, in another way:
// Suggest a group of tags depends on a given group of tags.
func (index *TagIndex) RelativeTags(tags []string, count int) (relative_tags []string) {
	return nil
}

// Number of "relative tags".
func (index *TagIndex) RelativeTagsCount(tags []string) int {
	return 0
}

func (index *TagIndex) RelativeTagsOptions(tags []string, count int, options *IndexOptions) (relative_tags []string) {
	return nil
}

// Suggest some tag that
func (index *TagIndex) RandomSuggestTags(tags []string, count int) (sugs []string) {
	return nil
}

// When you wonder if all the indexing jobs are all done
func (index *TagIndex) WaitAllIndexingDone() {
	return
}

type IndexOptions struct{}
