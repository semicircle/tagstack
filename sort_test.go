package tagstack

import (
	"fmt"
	"testing"
)

func TestFade(t *testing.T) {

	fmt.Println(fade_score(10, 1410652800)) // 2 month ago
	fmt.Println(fade_score(5, 1415957252))

	fmt.Println(fade_score(20, 1410652800)) // 2 month ago
	fmt.Println(fade_score(10, 1415957252))
}
