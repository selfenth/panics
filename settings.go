package panics

import (
	"log"
	"strings"
)

type ignoreLocationChecker = func(funcLine, fileLine string) bool

var (
	defaultSettigs *settings
	logger         *log.Logger

	ignoreStdLibLocaionChecker ignoreLocationChecker = ignoreContainPath{
		paths: []string{
			"/src/bufio/",
			"/src/archive/",
			"/src/compress/",
			"/src/container/",
			"/src/context/",
			"/src/crypto/",
			"/src/database/",
			"/src/debug/",
			"/src/embed/",
			"/src/encoding/",
			"/src/errors/",
			"/src/expvar/",
			"/src/flag/",
			"/src/fmt/",
			"/src/go/",
			"/src/hash/",
			"/src/html/",
			"/src/image/",
			"/src/index/",
			"/src/io/",
			"/src/log/",
			"/src/maps/",
			"/src/math/",
			"/src/mine/",
			"/src/net/",
			"/src/os/",
			"/src/path/",
			"/src/plugin/",
			"/src/reflect/",
			"/src/regexp/",
			"/src/runtime/",
			"/src/slices/",
			"/src/sort/",
			"/src/strconv/",
			"/src/strings/",
			"/src/sync/",
			"/src/syscall/",
			"/src/testing/",
			"/src/text/",
			"/src/time/",
			"/src/unicode/",
			"/src/unsafe/",
		},
	}.shouldIgnore
)

func init() {
	logger = log.Default()
	defaultSettigs = &settings{
		ignoreLocationCheckers: []ignoreLocationChecker{ignoreStdLibLocaionChecker},
		watch:                  simpleLog,
	}
}

func IgnoreStdLibLocaionChecker() ignoreLocationChecker { return ignoreStdLibLocaionChecker }

type settings struct {
	ignoreLocationCheckers []ignoreLocationChecker
	watch                  func(PanicInfo)
}

func Default() *settings                       { return defaultSettigs }
func (s *settings) newAction() action          { return action{a: s} }
func (s *settings) SetWatch(f func(PanicInfo)) { s.watch = f }
func (s *settings) SetIgnoreLocationChecker(checkers ...ignoreLocationChecker) {
	s.ignoreLocationCheckers = checkers
}

func simpleLog(info PanicInfo) {
	if info.Recoverer != "" {
		logger.Printf("panic(%d#%s) with error:%v. Stack:%s\n", info.SelectLocation.Depth, info.Recoverer, info.Error, info.Stack)
	} else {
		logger.Printf("panic(%d) with error:%v. Stack:%s\n", info.SelectLocation.Depth, info.Error, info.Stack)
	}
}

type ignoreContainPath struct {
	paths []string
}

func (c ignoreContainPath) shouldIgnore(funcLine, fileLine string) bool {
	for _, path := range c.paths {
		if strings.Contains(fileLine, path) {
			return true
		}
	}
	return false
}
