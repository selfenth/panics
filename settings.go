package panics

import (
	"context"
	"log"
	"strings"
	"sync"
)

type ignorePositionChecker = func(funcLine, fileLine string) bool

var (
	globalSettings   *staticSettings
	fallbackSettings *settings

	namedSettings sync.Map

	logger *log.Logger

	ignoreStdLibChecker ignorePositionChecker = ignoreContainPath{
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

	// Recover recover panics.
	Recover func()
	// RecoverWithContext recover panic with context, the context can be get from PanicInfo.Context.
	RecoverWithContext func(ctx context.Context)
	// Always the given `f` will always be executed. Use `AlwaysRef` if `f` may change.
	Always func(f func()) action
	// AlwaysRef the given `f` will always be executed. Use `Always` if `f` won't change.
	AlwaysRef func(f *func()) action
	// Succeed the given `f` will be executed if no panic recovered. Use `SucceedRef` if `f` may change.
	Succeed func(f func()) action
	// SucceedRef the given `f` will be executed if no panic recovered. Use `Succeed` if `f` won't change.
	SucceedRef func(f *func()) action
	// Panic the given `f` will be executed if a panic recovered. Use `PanicRef` if `f` may change.
	Panic func(f func(PanicInfo)) action
	// PanicRef the given `f` will be executed if a panic recovered. Use `Panic` if `f` won't change.
	PanicRef func(f *func(PanicInfo)) action
	// Alias set alias for this recover. We can get alias in PanicInfo.Alias.
	Alias func(alias string) action
	// Safe controls how the user functions are executed. If safe is given true, all user functions passed with Succeed/Panic/Always
	// will run in safe mode (panics will be recovered and handle with fallback settings which mostly won't fail). Or the
	// panic from user functions won't be recovered. It has a higher priority then Safe property on settings, so we can call
	// safe as true/false to overwrite the setting.
	Safe func(safe bool) action
	// WithExtra anything you want to get from panic info.
	WithExtra func(any) action
)

func init() {
	logger = log.Default()
	globalSettings = &staticSettings{s: Default()}
	fallbackSettings = Default()

	a := globalSettings.s.newAction()
	Recover = a.Recover
	RecoverWithContext = a.RecoverWithContext
	Always = a.Always
	AlwaysRef = a.AlwaysRef
	Succeed = a.Succeed
	SucceedRef = a.SucceedRef
	Panic = a.Panic
	PanicRef = a.PanicRef
	Alias = a.Alias
	Safe = a.Safe
	WithExtra = a.WithExtra
}

func IgnoreStdLibChecker() ignorePositionChecker { return ignoreStdLibChecker }

type settings struct {
	ignorePositionCheckers []ignorePositionChecker
	watch                  func(PanicInfo)
	safe                   bool
}

// Default return a default settings instance, which will discard panic info and filter standard libraries(it  may have unexpected situations or bad cases)
// when finding actual panic locations.
func Default() *settings {
	return &settings{
		ignorePositionCheckers: []ignorePositionChecker{ignoreStdLibChecker},
		watch:                  discard,
	}
}

// Use create action with the given settings.
func Use(s *settings) action { return s.newAction() }

// ByName create action with settings stored with the given name, default settings will be used if no settings can be found with the name.
func ByName(name string) action {
	return action{a: &nameLazySettings{name: name}}
}

// StoreSettings store a setting with name, then we can create action with method: ByName("xxx").
func StoreSettings(name string, a *settings) {
	namedSettings.Store(name, a)
}

func (s *settings) newAction() action { return action{a: &staticSettings{s: s}} }

// SetWatch set watch function to current settings. The watch function will be called on panic with analyzed information.
func (s *settings) SetWatch(f func(PanicInfo)) *settings { s.watch = f; return s }

// SetSafe call SetSafe on current settings. If safe is set true, all user functions passed with Succeed/Panic/Always
// will run in safe mode (panics will be recovered and handle with fallback settings which mostly won't fail). Or the
// panic from user functions won't be recovered.
func (s *settings) SetSafe(safe bool) *settings { s.safe = safe; return s }

// SetIgnorePositionChecker call SetIgnorePositionChecker on current settings. The checkers are used to find **business-related panic location**.
// e.g. If the we have a bad code: `fmt.Fprintf(nil, "%v", "a")`, if will panic when is executed with stack:
//
// *********************************************
//
// runtime error: invalid memory address or nil pointer dereference
//
// goroutine 1 [running]:
//
// main.recoverSimple()
//
//	/Users/selfenth/Code/go/src/github.com/selfenth/expir/main.go:19
//
// panic({0x10434b7a0?, 0x1043cd9d0?})
//
//	/Users/selfenth/.g/go/src/runtime/panic.go:770 +0x124
//
// fmt.Fprintf({0x0, 0x0}, {0x10431a760, 0x1}, {0x14000078f20, 0x1, 0x1})
//
//	/Users/selfenth/.g/go/src/fmt/print.go:225 +0x58
//
// main.main()
//
//	/Users/selfenth/Code/go/src/github.com/selfenth/expir/main.go:12 +0x74
//
// *********************************************
//
// The panic is directly caused by `fmt.Fprintf`, but the reason for this panic is in function main(), so the stack line of `fmt.Fprintf` should be ignored,
// a checker returns true if it thinks the given location should be ignored.
func (s *settings) SetIgnorePositionChecker(checkers ...ignorePositionChecker) *settings {
	s.ignorePositionCheckers = checkers
	return s
}

// SetWatch call SetWatch on default settings. The watch function will be called on panic with analyzed information.
func SetWatch(f func(PanicInfo)) { globalSettings.s.watch = f }

// SetSafe call SetSafe on default settings. If safe is set true, all user functions passed with Succeed/Panic/Always
// will run in safe mode (panics will be recovered and handle with fallback settings which mostly won't fail). Or the
// panic from user functions won't be recovered.
func SetSafe(safe bool) { globalSettings.s.safe = true }

// SetWatchWithSimpleLog call SetWatch on default settings with simpleLog function.
func SetWatchWithSimpleLog() { globalSettings.s.watch = SimpleLog }

// SetIgnoreLocationChecker call SetIgnoreLocationChecker on default settings. The checkers are used to find **business-related panic location**.
// e.g. If the we have a bad code: `fmt.Fprintf(nil, "%v", "a")`, if will panic when is executed with stack:
//
// *********************************************
//
// runtime error: invalid memory address or nil pointer dereference
//
// goroutine 1 [running]:
//
// main.recoverSimple()
//
//	/Users/selfenth/Code/go/src/github.com/selfenth/expir/main.go:19
//
// panic({0x10434b7a0?, 0x1043cd9d0?})
//
//	/Users/selfenth/.g/go/src/runtime/panic.go:770 +0x124
//
// fmt.Fprintf({0x0, 0x0}, {0x10431a760, 0x1}, {0x14000078f20, 0x1, 0x1})
//
//	/Users/selfenth/.g/go/src/fmt/print.go:225 +0x58
//
// main.main()
//
//	/Users/selfenth/Code/go/src/github.com/selfenth/expir/main.go:12 +0x74
//
// *********************************************
//
// The panic is directly caused by `fmt.Fprintf`, but the reason for this panic is in function main(), so the stack line of `fmt.Fprintf` should be ignored,
// a checker returns true if it thinks the given location should be ignored.
func SetIgnorePositionChecker(checkers ...ignorePositionChecker) {
	globalSettings.s.ignorePositionCheckers = checkers
}

// SimpleLog a simple watch function that print log with log.Default()
func SimpleLog(info PanicInfo) {
	if info.Alias != "" {
		logger.Printf("[WATCHER]panic(%d#%s) with error:%v. Stack:%s\n", info.Actual.Depth, info.Alias, info.Error, info.Stack)
	} else {
		logger.Printf("[WATCHER]panic(%d) with error:%v. Stack:%s\n", info.Actual.Depth, info.Error, info.Stack)
	}
}
func discard(info PanicInfo) {}

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

type settingsContainer interface {
	load() *settings
}

type staticSettings struct{ s *settings }

func (a *staticSettings) load() *settings { return a.s }

type nameLazySettings struct{ name string }

func (a *nameLazySettings) load() *settings {
	s, ok := namedSettings.Load(a.name)
	var s2 *settings
	if !ok {
		s2 = Default()
		namedSettings.Store(a.name, s2)
	} else {
		s2 = s.(*settings)
	}
	return s2
}
