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

func PTupleEquals(t *Tuple) {
	if t.Fst == 3 && t.Snd == 5 {
		fmt.Println("t.Fst == 3")
	} else {
		fmt.Println("t.Fst != 3")
	}
}
