package congo

import (
	"fmt"
	"testing"
)

const testPackage = "./testdata"

func TestExecute(t *testing.T) {
	config := &Config{}
	c, err := Load(config, testPackage)
	if err != nil {
		t.Fatalf("Config.Open: %v\n", err)
	}

	for _, name := range c.Funcs() {
		t.Run(fmt.Sprintf("%s", name), func(t *testing.T) {
			target := c.Target(name)
			res, err := c.Execute(name)
			if err != nil {
				t.Fatal(err)
			}
			if res.Coverage < target.MinCoverage {
				t.Errorf("failed to achieve the desired coverage: %.3f < %.3f", res.Coverage, target.MinCoverage)
			}
		})
	}
}
