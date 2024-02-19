package panics

import (
	"context"
	"runtime"
	"strconv"
	"strings"
)

const (
	panicBufSize = 64 << 10
)

var (
	unknownLoc = Position{FileLine: "UNKNOWN", FuncLine: "UNKNOWN:-1", File: "UNKNOWN", Function: "UNKNOWN", Depth: -1}
)

type action struct {
	a settingsContainer

	safe      *bool
	alias     string
	extra     any
	always    *func()
	onPanic   *func(PanicInfo)
	onSucceed *func()
}

// Recover recover panics.
func (a action) Recover() {
	panicErr := recover()
	a.postRecover(context.Background(), panicErr)
}

// RecoverWithContext recover panic with context, the context can be get from PanicInfo.Context.
func (a action) RecoverWithContext(ctx context.Context) {
	panicErr := recover()
	a.postRecover(ctx, panicErr)
}

func (a action) postRecover(ctx context.Context, panicErr any) {
	safe := a.needRunFallbackSafe()
	if panicErr != nil {
		buf := make([]byte, panicBufSize)
		buf = buf[:runtime.Stack(buf, false)]
		stackStr := string(buf[:runtime.Stack(buf, false)])
		locs := a.findPanics(stackStr, panicErr)
		for _, loc := range locs {
			info := PanicInfo{
				Direct:  loc.Direct,
				Actual:  loc.Actual,
				Error:   panicErr,
				Stack:   stackStr,
				Alias:   a.alias,
				Context: ctx,
				Extra:   a.extra,
			}
			if safe {
				fallbackSafeRunWithInfo(ctx, &a.a.load().watch, info)
				fallbackSafeRunWithInfo(ctx, a.onPanic, info)
			} else {
				if a.a.load().watch != nil {
					a.a.load().watch(info)
				}
				if a.onPanic != nil && *a.onPanic != nil {
					(*a.onPanic)(info)
				}
			}
		}
	} else if safe {
		fallbackSafeRun(ctx, a.onSucceed)
	} else if a.onSucceed != nil && *a.onSucceed != nil {
		(*a.onSucceed)()
	}
	if safe {
		fallbackSafeRun(ctx, a.always)
	} else if a.always != nil && *a.always != nil {
		(*a.always)()
	}
}

func (a action) needRunFallbackSafe() bool {
	if a.safe != nil {
		// safe setting on action has higher priority
		return *a.safe
	}
	return a.a.load().safe
}

// Always the given `f` will always be executed. Use `AlwaysStatic` if `f` won't change.
func (a action) Always(f *func()) action { a.always = f; return a }

// AlwaysStatic the given `f` will always be executed. Use `Always` if `f` may change.
func (a action) AlwaysStatic(f func()) action { a.always = &f; return a }

// Succeed the given `f` will be executed if no panic recovered. Use `SucceedStatic` if `f` won't change.
func (a action) Succeed(f *func()) action { a.onSucceed = f; return a }

// SucceedStatic the given `f` will be executed if no panic recovered. Use `Succeed` if `f` may change.
func (a action) SucceedStatic(f func()) action { a.onSucceed = &f; return a }

// Panic the given `f` will be executed if a panic recovered. Use `PanicStatic` if `f` won't change.
func (a action) Panic(f *func(PanicInfo)) action { a.onPanic = f; return a }

// PanicStatic the given `f` will be executed if a panic recovered. Use `Panic` if `f` may change.
func (a action) PanicStatic(f func(PanicInfo)) action { a.onPanic = &f; return a }

// Safe controls how the user functions be executed. If safe is given true, all user functions passed with Succeed/Panic/Always
// will run in safe mode (panics will be recovered and handle with fallback settings which mostly won't fail). Or the
// panic from user functions won't be recovered. It has a higher priority then Safe property on settings, so we can call
// safe as true/false to overwrite the setting.
func (a action) Safe(safe bool) action { a.safe = &safe; return a }

// Alias set alias for this recover. We can get alias in PanicInfo.Alias so we can known where the panic is recoverd.
func (a action) Alias(alias string) action { a.alias = alias; return a }

// WithExtra anything you want to get from panic info.
func (a action) WithExtra(extra any) action { a.extra = extra; return a }

type Position struct {
	FuncLine string // the raw string of function line in the stack
	FileLine string // the raw string of file line in the stack

	File     string // file path
	Line     int64  // line number
	Function string // function name
	/*
	   we may recover a panic that caused by the logic in another recover function, like the following stack:

	   goroutine 1 [running]:
	   main.recoverSimple()
	           /Users/selfenth/Code/go/src/github.com/selfenth/expir/main.go:26 +0x5c
	   panic({0x10060cdc0, 0x100620f80})
	           /Users/selfenth/.g/go/src/runtime/panic.go:838 +0x204
	   main.recoverThenPanic()
	           /Users/selfenth/Code/go/src/github.com/selfenth/expir/main.go:38 +0xf0
	   panic({0x10060cdc0, 0x100620f70})
	           /Users/selfenth/.g/go/src/runtime/panic.go:838 +0x204
	   main.main()
	           /Users/selfenth/Code/go/src/github.com/selfenth/expir/main.go:19 +0x194

	   we can get 2 panic infos from this stack, one caused by `main.go:38` has Depth 0, and the other caused by `main.go:19` has Depth 1
	*/
	Depth int
}

type PanicInfo struct {
	Direct Position // the code position that directly caseud this panic
	Actual Position // the code position that actually caused this panic

	Stack   string          // the stack dumps for this panic
	Error   any             // the object which is got by recover()
	Context context.Context // the argument that pass to RecoverWithContext, or context.Background if called with Recover
	Alias   string          // the alias of the code position that called Recover/RecoverWithContext
	Extra   any             // the paramater pass to WithExtra method
}

func (s *action) findPanics(stack string, err any) []struct {
	Direct Position
	Actual Position
} {
	/*
	   goroutine 1 [running]:r
	   main.recoverSimple()
	           /Users/selfenth/Code/go/src/github.com/selfenth/expir/main.go:26 +0x5c
	   panic({0x10060cdc0, 0x100620f80})
	           /Users/selfenth/.g/go/src/runtime/panic.go:838 +0x204
	   main.recoverThenPanic()
	           /Users/selfenth/Code/go/src/github.com/selfenth/expir/main.go:38 +0xf0
	   panic({0x10060cdc0, 0x100620f70})
	           /Users/selfenth/.g/go/src/runtime/panic.go:838 +0x204
	   main.main()
	           /Users/selfenth/Code/go/src/github.com/selfenth/expir/main.go:19 +0x194
	*/
	var (
		i     = 0
		lines = strings.Split(stack, "\n")
	)
	panicLocs := make([]struct {
		Direct Position
		Actual Position
	}, 0, 1)

	var directLoc *Position
	for i < len(lines) {
		if directLoc != nil {
			// 有Direct的，说明目前在查找Select的位置
			if s.isIgnoreLoc(lines[i-1], lines[i]) {
				// 检测当前行是否是被忽略的，如果被忽略的话，往后跳两行
				i += 2
			} else {
				// 当前行被选中
				panicLocs = append(panicLocs, struct {
					Direct Position
					Actual Position
				}{
					Direct: *directLoc,
					Actual: s.parseLocation(lines[i-1], lines[i]),
				})
				directLoc, i = nil, i+1 // 重置，让后续继续查找panic
			}
		} else if strings.HasPrefix(lines[i], "panic(") {
			directLoc, i = ptrOf(s.parseLocation(lines[i+2], lines[i+3])), i+3 // 跳到下下个文件行，开始位置查找
		} else {
			i += 1 // 跳到下一个方法行
		}
	}
	if directLoc != nil {
		// 找到了direct的，没能找到select的，使用direct的兜底
		panicLocs = append(panicLocs, struct {
			Direct Position
			Actual Position
		}{
			Direct: *directLoc,
			Actual: *directLoc,
		})
	} else if len(panicLocs) == 0 {
		// 整个堆栈扫下来没有找到panics，理论上不应该存在，这里用特殊内容兜下
		return []struct {
			Direct Position
			Actual Position
		}{{Direct: unknownLoc, Actual: unknownLoc}}
	}
	for i := range panicLocs {
		panicLocs[i].Direct.Depth, panicLocs[i].Actual.Depth = i, i
	}
	return panicLocs
}
func (s *action) isIgnoreLoc(funcLine, fileLine string) bool {
	for _, check := range s.a.load().ignoreLocationCheckers {
		if check(funcLine, fileLine) {
			return true
		}
	}
	return false
}
func (s *action) parseLocation(funcLine, fileLine string) Position {
	funcName := funcLine
	if idx := strings.Index(funcLine, "("); idx > 0 {
		funcName = funcLine[:idx]
	}
	parts := strings.SplitN(strings.TrimSpace(fileLine), ":", 2)
	if len(parts) == 1 {
		return Position{Function: funcName, File: parts[0], Line: -1, FuncLine: funcLine, FileLine: strings.TrimSpace(fileLine)}
	}
	line, err := strconv.ParseInt(strings.Split(parts[1], " ")[0], 10, 64)
	if err != nil {
		line = -1
	}
	return Position{Function: funcName, File: parts[0], Line: line, FuncLine: funcLine, FileLine: strings.TrimSpace(fileLine)}
}

func ptrOf[T any](v T) *T { return &v }

func fallbackSafeRun(ctx context.Context, f *func()) {
	if f == nil {
		return
	}

	defer Use(fallbackSettings).RecoverWithContext(ctx)

	(*f)()
}

func fallbackSafeRunWithInfo(ctx context.Context, f *func(PanicInfo), info PanicInfo) {
	if f == nil || *f == nil {
		return
	}

	defer Use(fallbackSettings).RecoverWithContext(ctx)

	(*f)(info)
}
