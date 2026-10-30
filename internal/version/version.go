package version

import "fmt"

var Version = "dev"

func String() string {
	return fmt.Sprintf("toolcheck %s", Version)
}
