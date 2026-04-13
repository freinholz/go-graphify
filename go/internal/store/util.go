package store

import (
	"fmt"
	"time"
)

func timestamp() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
