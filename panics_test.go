package panics

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	panicsTestGoFile = "github.com/selfenth/panics/panics_test.go"
	panicsPkg        = "github.com/selfenth/panics"
)

func TestRecoverWorks(t *testing.T) {
	defer Recover()

	panic("a")
}

func TestRecoverWithContextWorks(t *testing.T) {
	defer RecoverWithContext(context.Background())

	panic("a")
}

func TestRecoverWithActions(t *testing.T) {
	t.Run("Paniced", func(t *testing.T) {
		var (
			alwaysCalled, succeedCalled bool
			info                        PanicInfo
		)
		func() {
			defer Alias("loc1").PanicStatic(func(pi PanicInfo) { info = pi }).AlwaysStatic(func() { alwaysCalled = true }).
				SucceedStatic(func() { succeedCalled = true }).Recover()

			panic("a")
		}()
		assert.True(t, alwaysCalled)
		assert.False(t, succeedCalled)
		assert.Equal(t, "loc1", info.Alias)

		assert.True(t, len(info.Direct.FileLine) > 0)
		assert.True(t, len(info.Direct.FuncLine) > 0)
		assert.Equal(t, panicsPkg+".TestRecoverWithActions.func1.1", info.Direct.Function)
		assert.True(t, strings.HasSuffix(info.Direct.File, panicsTestGoFile))
		assert.Equal(t, 0, info.Direct.Depth)

		assert.True(t, len(info.Actual.FileLine) > 0)
		assert.True(t, len(info.Actual.FuncLine) > 0)
		assert.Equal(t, panicsPkg+".TestRecoverWithActions.func1.1", info.Actual.Function)
		assert.True(t, strings.HasSuffix(info.Actual.File, panicsTestGoFile))
		assert.Equal(t, 0, info.Actual.Depth)
	})
	t.Run("NotPanic", func(t *testing.T) {
		var (
			alwaysCalled, succeedCalled bool
			info                        PanicInfo
		)
		func() {
			defer Alias("loc1").PanicStatic(func(pi PanicInfo) { info = pi }).AlwaysStatic(func() { alwaysCalled = true }).SucceedStatic(func() { succeedCalled = true }).Recover()
		}()
		assert.True(t, alwaysCalled)
		assert.True(t, succeedCalled)
		assert.Equal(t, "", info.Alias)

		assert.True(t, len(info.Direct.FileLine) == 0)
		assert.True(t, len(info.Direct.FuncLine) == 0)
		assert.Equal(t, "", info.Direct.Function)
		assert.Equal(t, "", info.Direct.File)
		assert.Equal(t, 0, info.Direct.Depth)

		assert.True(t, len(info.Actual.FileLine) == 0)
		assert.True(t, len(info.Actual.FuncLine) == 0)
		assert.Equal(t, "", info.Actual.Function)
		assert.Equal(t, "", info.Actual.File)
		assert.Equal(t, 0, info.Actual.Depth)
	})
}

func TestRecoverCanModifyFunc(t *testing.T) {
	t.Run("Paniced", func(t *testing.T) {
		onPanic := func(pi PanicInfo) {}
		alwaysF := func() {}
		onSucceed := func() {}

		var (
			alwaysCalled, succeedCalled bool
			info                        PanicInfo
		)

		func() {
			defer Alias("loc1").Panic(&onPanic).Always(&alwaysF).Succeed(&onSucceed).Recover()

			onPanic = func(pi PanicInfo) { info = pi }
			alwaysF = func() { alwaysCalled = true }
			onSucceed = func() { succeedCalled = true }

			panic("a")
		}()

		assert.True(t, alwaysCalled)
		assert.False(t, succeedCalled)
		assert.True(t, strings.HasSuffix(info.Direct.File, panicsTestGoFile))
		assert.True(t, strings.HasSuffix(info.Actual.File, panicsTestGoFile))

	})
	t.Run("NotPanic", func(t *testing.T) {
		onPanic := func(pi PanicInfo) {}
		alwaysF := func() {}
		onSucceed := func() {}

		var (
			alwaysCalled, succeedCalled bool
			info                        PanicInfo
		)

		func() {
			defer Alias("loc1").Panic(&onPanic).Always(&alwaysF).Succeed(&onSucceed).Recover()

			onPanic = func(pi PanicInfo) { info = pi }
			alwaysF = func() { alwaysCalled = true }
			onSucceed = func() { succeedCalled = true }
		}()

		assert.True(t, alwaysCalled)
		assert.True(t, succeedCalled)
		assert.True(t, info.Direct.File == "")
		assert.True(t, info.Actual.File == "")
	})
}
func TestRecoverWorksUseNewSettings(t *testing.T) {
	var info PanicInfo
	a := Use(Default().SetWatch(func(pi PanicInfo) { info = pi }))

	func() {
		defer a.Recover()
		panic("a")
	}()
	assert.True(t, strings.HasSuffix(info.Direct.File, panicsTestGoFile))
	assert.True(t, strings.HasSuffix(info.Actual.File, panicsTestGoFile))
}

func TestRecoverWithContextWorksUseNewSettings(t *testing.T) {
	var info PanicInfo
	a := Use(Default().SetWatch(func(pi PanicInfo) { info = pi }))

	func() {
		defer a.RecoverWithContext(context.Background())
		panic("a")
	}()
	assert.True(t, strings.HasSuffix(info.Direct.File, panicsTestGoFile))
	assert.True(t, strings.HasSuffix(info.Actual.File, panicsTestGoFile))
}

func TestSafe(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		assert.Panics(t, func() {
			defer SucceedStatic(func() { panic("SucceedStatic") }).Recover()
		})
		assert.Panics(t, func() {
			defer AlwaysStatic(func() { panic("AlwaysStatic") }).Recover()
		})
		assert.Panics(t, func() {
			defer PanicStatic(func(PanicInfo) { panic("PanicStatic") }).Recover()
			panic("a")
		})
	})
	t.Run("DefaultWithActionSafe", func(t *testing.T) {
		func() {
			defer SucceedStatic(func() { panic("SucceedStatic") }).Safe(true).Recover()
		}()
		func() {
			defer AlwaysStatic(func() { panic("AlwaysStatic") }).Safe(true).Recover()
		}()
		func() {
			defer PanicStatic(func(PanicInfo) { panic("PanicStatic") }).Safe(true).Recover()
			panic("a")
		}()
	})
	t.Run("NewSettings", func(t *testing.T) {
		a := Use(Default())
		assert.Panics(t, func() {
			defer a.SucceedStatic(func() { panic("SucceedStatic") }).Recover()
		})
		assert.Panics(t, func() {
			defer a.AlwaysStatic(func() { panic("AlwaysStatic") }).Recover()
		})
		assert.Panics(t, func() {
			defer a.PanicStatic(func(PanicInfo) { panic("PanicStatic") }).Recover()
			panic("a")
		})
	})
	t.Run("NewSettingsWithActionSafe", func(t *testing.T) {
		a := Use(Default())
		func() {
			defer a.SucceedStatic(func() { panic("SucceedStatic") }).Safe(true).Recover()
		}()
		func() {
			defer a.AlwaysStatic(func() { panic("AlwaysStatic") }).Safe(true).Recover()
		}()
		func() {
			defer a.PanicStatic(func(PanicInfo) { panic("PanicStatic") }).Safe(true).Recover()
			panic("a")
		}()
	})
	t.Run("NewSafeSettings", func(t *testing.T) {
		a := Use(Default().SetSafe(true))
		func() {
			defer a.SucceedStatic(func() { panic("SucceedStatic") }).Recover()
		}()
		func() {
			defer a.AlwaysStatic(func() { panic("AlwaysStatic") }).Recover()
		}()
		func() {
			defer a.PanicStatic(func(PanicInfo) { panic("PanicStatic") }).Recover()
			panic("a")
		}()
	})
	t.Run("NewSafeSettingsWithActionSafe", func(t *testing.T) {
		a := Use(Default().SetSafe(true))
		func() {
			defer a.SucceedStatic(func() { panic("SucceedStatic") }).Safe(true).Recover()
		}()
		func() {
			defer a.AlwaysStatic(func() { panic("AlwaysStatic") }).Safe(true).Recover()
		}()
		func() {
			defer a.PanicStatic(func(PanicInfo) { panic("PanicStatic") }).Safe(true).Recover()
			panic("a")
		}()
	})
	t.Run("NewSafeSettingsWithActionUnsafe", func(t *testing.T) {
		a := Use(Default().SetSafe(true))
		assert.Panics(t, func() {
			defer a.SucceedStatic(func() { panic("SucceedStatic") }).Safe(false).Recover()
		})
		assert.Panics(t, func() {
			defer a.AlwaysStatic(func() { panic("AlwaysStatic") }).Safe(false).Recover()
		})
		assert.Panics(t, func() {
			defer a.PanicStatic(func(PanicInfo) { panic("PanicStatic") }).Safe(false).Recover()
			panic("a")
		})
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("NilSucceedPtr", func(t *testing.T) {
		defer Succeed(nil).Recover()
	})
	t.Run("NilAlwaysPtr", func(t *testing.T) {
		defer Always(nil).Recover()
	})
	t.Run("NilPanicPtr", func(t *testing.T) {
		defer Panic(nil).Recover()
		panic("a")
	})
}

func TestDefaultRecoverCanIgnoreStdLib(t *testing.T) {
	var info PanicInfo
	func() {
		defer PanicStatic(func(pi PanicInfo) { info = pi }).Recover()

		fmt.Fprint(nil, 1)
	}()

	assert.True(t, strings.HasSuffix(info.Direct.File, "/src/fmt/print.go"))
	assert.Equal(t, "fmt.Fprint", info.Direct.Function)

	assert.True(t, strings.HasSuffix(info.Actual.File, panicsTestGoFile))
	assert.Equal(t, panicsPkg+".TestDefaultRecoverCanIgnoreStdLib.func1", info.Actual.Function)
}

func TestNestedPanicsCanBeAnalyzed(t *testing.T) {
	infos := []PanicInfo{}
	func() {
		defer Alias("1").PanicStatic(func(pi PanicInfo) { infos = append(infos, pi) }).Recover()

		defer Alias("2").PanicStatic(func(pi PanicInfo) { panic("a") }).Recover()
		fmt.Fprint(nil, 1)
	}()
	assert.Len(t, infos, 2)
	assert.Equal(t, "1", infos[0].Alias)
	assert.Equal(t, 0, infos[0].Actual.Depth)
	assert.Equal(t, 0, infos[0].Direct.Depth)

	assert.Equal(t, "1", infos[1].Alias)
	assert.Equal(t, 1, infos[1].Actual.Depth)
	assert.Equal(t, 1, infos[1].Direct.Depth)
}
