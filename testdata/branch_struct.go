package testdata

import (
	"fmt"
)

type Hoge struct {
	Foo int
	Bar string
}

func BranchStruct(hoge Hoge) {
	if hoge.Foo == 1 && hoge.Bar == "bar" {
		fmt.Println("OK")
	} else {
		fmt.Println("NG")
	}
}

func main() {
	BranchStruct(struct {
		Foo int
		Bar string
	}{Foo: 100, Bar: "hoge"})
}
