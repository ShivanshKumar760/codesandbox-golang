// Compiled once while the runner image is built, only to fill Go's build
// cache with common packages. User programs that import them then compile fast.
package main

import (
	_ "bufio"
	_ "bytes"
	_ "container/heap"
	_ "context"
	_ "encoding/json"
	_ "errors"
	_ "fmt"
	_ "io"
	_ "math"
	_ "math/big"
	_ "math/rand"
	_ "net/http"
	_ "os"
	_ "regexp"
	_ "sort"
	_ "strconv"
	_ "strings"
	_ "sync"
	_ "time"
	_ "unicode"
)

func main() {}
