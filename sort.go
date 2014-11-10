package tagstack

import (
	"math"
)

const (
	reddit_factor = float64(45000 * 2 * 30)
)

func fade_score(score float64, date uint64) float64 {
	order := math.Log10(math.Max(math.Abs(score), 1))
	var sign float64
	if score > 0 {
		sign = 1
	} else if score < 0 {
		sign = -1
	} else {
		sign = 0
	}
	seconds := float64(date - 1288465200)
	return sign*order + seconds/reddit_factor
}

// TOOD: cache.
func confidence_score(up, down int) float64 {
	n := float64(up + down)
	if n == 0 {
		return 0
	}

	z := float64(1.281551565545)
	p := float64(up) / n

	left := p + 1/(2*n)*z*z
	right := z * math.Sqrt(p*(1-p)/n+z*z/(4*n*n))
	under := 1 + 1/n*z*z

	return (left - right) / under
}
