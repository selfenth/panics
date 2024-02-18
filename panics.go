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
	unknownLoc = Location{FileLine: "UNKNOWN", FuncLine: "UNKNOWN:-1", File: "UNKNOWN", Function: "UNKNOWN", Depth: -1}
)

type action struct {
	a *settings

	alias     string
	always    *func()
	onPanic   *func(PanicInfo)
	onSucceed *func()
}

func (a action) Alias(alias string) action { a.alias = alias; return a }

// Recover
func (a action) Recover() {
	if panicErr := recover(); panicErr != nil {
		buf := make([]byte, panicBufSize)
		stackStr := string(buf[:runtime.Stack(buf, false)])

		locs := a.findPanics(stackStr, panicErr)
		for _, loc := range locs {
			info := PanicInfo{DirectLocation: loc.Direct, SelectLocation: loc.Select, Error: panicErr, Stack: stackStr, Recoverer: a.alias}
			if a.onPanic != nil {
				(*a.onPanic)(info)
			}
			if a.a.watch != nil {
				a.a.watch(info)
			}
		}
	}
}

// RecoverWithContext
func (a action) RecoverWithContext(ctx context.Context) {
	if panicErr := recover(); panicErr != nil {
		buf := make([]byte, panicBufSize)
		buf = buf[:runtime.Stack(buf, false)]
		stackStr := string(buf[:runtime.Stack(buf, false)])

		locs := a.findPanics(stackStr, panicErr)
		for _, loc := range locs {
			info := PanicInfo{DirectLocation: loc.Direct, SelectLocation: loc.Select, Error: panicErr, Stack: stackStr, Recoverer: a.alias}
			if a.onPanic != nil {
				(*a.onPanic)(info)
			}
			if a.a.watch != nil {
				a.a.watch(info)
			}
		}
	}
}

func (a action) Always(f *func()) action              { a.always = f; return a }
func (a action) AlwaysStatic(f func()) action         { a.always = &f; return a }
func (a action) Succeed(f *func()) action             { a.onSucceed = f; return a }
func (a action) SucceedStatic(f func()) action        { a.onSucceed = &f; return a }
func (a action) Panic(f *func(PanicInfo)) action      { a.onPanic = f; return a }
func (a action) PanicStatic(f func(PanicInfo)) action { a.onPanic = &f; return a }

type Location struct {
	FuncLine string // 堆栈中方法信息的行内容
	FileLine string // 堆栈中文件信息的行内容

	File     string // 文件路径
	Line     int64  // 文件行号
	Function string // 方法名
	Depth    int    // 可能panic是连发的，recover后又发生panic，这个depth如果是0，标识是最后一次的panic，1为倒数第二次panic，以此类推
}

type PanicInfo struct {
	DirectLocation Location // 直接发生panic的位置信息
	SelectLocation Location // 筛选后的panic的位置信息，可能不对应直接发生panic的位置信息（比如过滤了标准库的位置，使用调用标准库方法的位置），但是是业务关注的位置信息

	Stack     string          // 捕获时候的堆栈信息
	Error     any             // 捕获时候的错误数据
	Context   context.Context // 如果是通过RecoverWithContext执行的，会传递过去，否则为nil
	Recoverer string          // 执行Recover的位置alias
}

func (s *action) findPanics(stack string, err any) []struct {
	Direct Location
	Select Location
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
		Direct Location
		Select Location
	}, 0, 1)

	var directLoc *Location
	for i < len(lines) {
		if directLoc != nil {
			// 有Direct的，说明目前在查找Select的位置
			if s.isIgnoreLoc(lines[i-1], lines[i]) {
				// 检测当前行是否是被忽略的，如果被忽略的话，往后跳两行
				i += 2
			} else {
				// 当前行被选中
				panicLocs = append(panicLocs, struct {
					Direct Location
					Select Location
				}{
					Direct: *directLoc,
					Select: s.parseLocation(lines[i-1], lines[i]),
				})
				directLoc, i = nil, i+1 // 重置，让后续继续查找panic
			}
		} else if strings.HasPrefix(lines[i], "panic(") {
			directLoc, i = ptrOf(s.parseLocation(lines[i-1], lines[i])), i+3
		} else {
			i += 2
		}
	}
	if directLoc != nil {
		// 找到了direct的，没能找到select的，使用direct的兜底
		panicLocs = append(panicLocs, struct {
			Direct Location
			Select Location
		}{
			Direct: *directLoc,
			Select: *directLoc,
		})
	} else if len(panicLocs) == 0 {
		// 整个堆栈扫下来没有找到panics，理论上不应该存在，这里用特殊内容兜下
		return []struct {
			Direct Location
			Select Location
		}{{Direct: unknownLoc, Select: unknownLoc}}
	}
	for i := range panicLocs {
		panicLocs[i].Direct.Depth, panicLocs[i].Select.Depth = i, i
	}
	return panicLocs
}
func (s *action) isIgnoreLoc(funcLine, fileLine string) bool {
	for _, check := range s.a.ignoreLocationCheckers {
		if check(funcLine, fileLine) {
			return true
		}
	}
	return false
}
func (s *action) parseLocation(funcLine, fileLine string) Location {
	funcName := funcLine
	if idx := strings.Index(funcLine, "("); idx > 0 {
		funcName = funcLine[:idx]
	}
	parts := strings.SplitN(strings.TrimSpace(fileLine), ":", 2)
	if len(parts) == 1 {
		return Location{Function: funcName, File: parts[0], Line: -1, FuncLine: funcLine, FileLine: fileLine}
	}
	line, err := strconv.ParseInt(strings.Split(parts[1], " ")[0], 10, 64)
	if err != nil {
		line = -1
	}
	return Location{Function: funcName, File: parts[0], Line: line, FuncLine: funcLine, FileLine: fileLine}
}

var Recover = defaultSettigs.newAction().Recover
var RecoverWithContext = defaultSettigs.newAction().RecoverWithContext
var Always = defaultSettigs.newAction().Always
var AlwaysStatic = defaultSettigs.newAction().Always
var Succeed = defaultSettigs.newAction().Always
var SucceedStatic = defaultSettigs.newAction().Always
var Panic = defaultSettigs.newAction().Always
var PanicStatic = defaultSettigs.newAction().Always
var Alias = defaultSettigs.newAction().Alias

func ptrOf[T any](v T) *T { return &v }
