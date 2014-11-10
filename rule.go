package tagstack

import (
	"sort"
)

// The rule struct: This should be configured very carefully.
// It's normal to use a json.Decode to generate this.
type Rule struct {
	Normalization map[string][]string
	Entanglement  [][]string
	Containing    map[string][]string
}

type rule struct {
	// from unnormal to normal map.
	norm_map map[string]string
	// 'one for all' map.
	entg_map map[string][]string
	// the 'up chan' map.
	contain_map map[string][]string
}

func (r *Rule) init() *rule {
	ret := &rule{}

	// reverse the map.
	ret.norm_map = make(map[string]string)
	for normal_form, unnormals := range r.Normalization {
		for _, unnormal_form := range unnormals {
			ret.norm_map[unnormal_form] = normal_form
		}
	}

	// reverse the map.
	ret.entg_map = make(map[string][]string)
	for _, group := range r.Entanglement {
		for _, tag := range group {
			ret.entg_map[tag] = group
		}
	}

	// reverse the map, then expand each to the full set of the contained items.
	basic_contain_map := make(map[string][]string)
	for upper, lowers := range r.Containing {
		for _, lower := range lowers {
			if forks, ok := basic_contain_map[lower]; !ok {
				basic_contain_map[lower] = []string{upper}
			} else {
				basic_contain_map[lower] = append(forks, upper)
			}
		}
	}
	DebugLogger.Printf("basic_contain_map: %+v", basic_contain_map)
	ret.contain_map = make(map[string][]string)
	for lowest, upperset := range basic_contain_map {
		plain := make([]string, len(upperset))
		copy(plain, upperset)
		DebugLogger.Printf("lowest: %v, plain: %+v, upperset: %+v", lowest, plain, upperset)
		for i := 0; i < len(plain); i++ {
			curr := plain[i]
			// Is this element duplicated with a previous?
			duplicated_i := false
			for j := 0; j < i; j++ {
				if curr == plain[j] {
					duplicated_i = true
					break
				}
			}
			if duplicated_i {
				// remove this element
				plain = append(plain[:i], plain[i+1:]...)
				i--
				continue
			}

			// expand this:
			if forks, ok := basic_contain_map[curr]; ok {
				plain = append(plain, forks...)
			}
		}
		ret.contain_map[lowest] = plain
	}

	return ret
}

// tag info for indexing.
type taginfo struct {
	title      string
	score      float64
	enrelative bool // enable RelativeTags feature.

	// the apply part:
	disabled     bool // to mark this field is disabled, won't affect the index.
	aliases      []string
	alias_scores []float64
}

// apply the indexing rules on the tags: add aliases / remove duplicated tags and aliases.
func (r *rule) applyRulesForIndexing(infos []*taginfo) []*taginfo {
	// normalization.
	for _, info := range infos {
		// has a norm_form ?
		if norm_form, ok := r.norm_map[info.title]; ok {
			// if the info's normal form is duplicated with another taginfo.title, mark the info as disabled.
			for _, info2 := range infos {
				if info != info2 && norm_form == info2.title {
					info.disabled = true
					break
				}
			}
			// if the info is not disabled, change it to the normal form.
			if !info.disabled {
				info.title = norm_form
			}
		}
	}

	// containing.
	// TODO: efficiency: sorting both info & uppers before matching.
	for _, info := range infos {
		if !info.disabled {
			if uppers, ok := r.contain_map[info.title]; ok {
				// set alias.
				info.aliases = make([]string, len(uppers))
				copy(info.aliases, uppers)
				// if upper is one of the info.titles ? mark the info as disabled.
				for _, upper := range uppers {
					for _, info2 := range infos {
						if info != info2 && upper == info2.title {
							info2.disabled = true
							break
						}
					}
				}
			}
		}
	}

	// entanglement.
	for _, info := range infos {
		if !info.disabled {
			if aliases, ok := r.entg_map[info.title]; ok {
				// add the aliases to the unit.
				info.aliases = append(info.aliases, aliases...)
			}
		}
	}

	// remove disabled ones.
	ret := make([]*taginfo, 0, len(infos))
	for _, info := range infos {
		if !info.disabled {
			ret = append(ret, info)
		}
	}

	return ret
}

func (r *rule) applyRulesForSearching(tags []string) []string {
	tagMap := make(map[string]bool)
	for _, tag := range tags {
		if norm_form, ok := r.norm_map[tag]; ok {
			if _, ok := tagMap[norm_form]; ok {
				continue
			} else {
				tagMap[norm_form] = true
			}
		} else {
			tagMap[tag] = true
		}
	}
	ret := make([]string, 0, len(tagMap))
	for tag, _ := range tagMap {
		ret = append(ret, tag)
	}
	sort.Strings(ret)
	return ret
}

// taginfo sorting:
type taginfo_title_sorter []*taginfo

func (s taginfo_title_sorter) Len() int           { return len(s) }
func (s taginfo_title_sorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s taginfo_title_sorter) Less(i, j int) bool { return s[i].title < s[j].title }
