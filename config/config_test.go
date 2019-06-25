package config

import (
	"fmt"
	"testing"
)

func TestInitConfig(t *testing.T) {
	cfg := InitConfig("qt", `C:\Users\zhouyc\gospace\src\github.com\MrChang666\qt\config`)
	fmt.Println(cfg.LogLevel)
	fmt.Println(cfg.Symbols)
}
