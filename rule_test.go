package tagstack

import (
	"log"
	"testing"
)

func dummyRule() *Rule {
	rt := &Rule{}

	rt.Normalization = make(map[string][]string)
	rt.Normalization["美食"] = []string{"吃", "好吃"}
	rt.Normalization["住宿"] = []string{"住"}

	rt.Entanglement = make([][]string, 3)
	rt.Entanglement[0] = []string{"住宿", "酒店", "旅馆"}
	rt.Entanglement[1] = []string{"骑行", "骑车", "自行车"}
	rt.Entanglement[2] = []string{"南锣", "南锣鼓巷"}

	rt.Containing = make(map[string][]string)
	rt.Containing["美食"] = []string{"小吃", "甜点", "西餐"}
	rt.Containing["西餐"] = []string{"马卡龙", "牛排", "烤肉"}
	rt.Containing["烧烤"] = []string{"夜烧烤", "烤肉"}
	rt.Containing["徒搭"] = []string{"徒步", "搭车"}

	return rt
}

func Test1(t *testing.T) {
	rt := dummyRule()
	lr := rt.init()
	// log.Printf("%+v", rt)
	log.Printf("%+v", lr)
}
