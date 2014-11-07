package tagstack

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

// normalization
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
			// Is this element already in the previous?
			for j := 0; j < i; j++ {
				if curr == plain[j] {
					// remove this element
					plain = append(plain[:j], plain[j+1:]...)
				}
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
