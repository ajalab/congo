package testdata

import (
	"fmt"
)

type Tuple struct {
	Fst int
	Snd int
}

func TupleEquals(t Tuple) {
	if t.Fst == t.Snd {
		fmt.Println("t.Fst == t.Snd")
	} else {
		fmt.Println("t.Fst != t.Snd")
	}
}
