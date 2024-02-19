# panics
---

## 为什么要写这样一个库？
一个golang处理panics的工具库。

一般我们会怎么写panic呢？极简的大概长下面这样：
```golang
func func1() {
    defer func() {
        if err := recover(); err != nil {
            handlePanic(err)
        }
    }()

    ...
}
```
说实话还是比较冗长的，如果能够写出如下形式是否会更加优雅：
```golang
func func1() {
    defer panics.Recover()
    ...
}
```
看到这里大家肯定会有个问题：`handlePanic(err)` 去哪儿了？大家考虑个问题，一个项目中，我们处理panic的逻辑是否都是基本一致的呢？大部分是不是都是：
```golang
func handlePanic(err any) {
    stack := getStack()
    doLog(err, stack)
    doMetrics(err, stack, "some information")
}
```
那么我们其实可以把这些统一的处理通过统一的设置来实现。
```golang
func init() {
    panics.SetWatch(func (info panics.PanicInfo) {
        doLog(info.Error, info.Stack)
        doMetrics(info.Error, info.Stack, info.Extra, info.Alias)
    })
}
```
完成了这样的设置之后，我们就可以如上，仅通过一行完成Recover。

但是实际除了这样统一的处理，我们还会存在一些不同的处理行为。如：
```golang
func func1() (val int, err error) {
    wg := sync.WaitGroup{}
    wg.Add(1)
    defer func() {
        wg.Done() // 始终需要执行的
        if panicObj := recover(); panicObj != nil {
            err, val = fmt.Errorf("panic with:%v", panicObj), -1 // 发生panic时候需要执行的
            handlePanic(panicObj)
        } else {
            val = 1 // 执行成功时设置
        }
    }()
    ...
}
```
可以看到，在一个defer的方法中，大致可以分为三类的行为：
- 不管是否panic都需要执行的
- 发生panic需要执行的
- 未发生panic需要执行的
一次recover可能存在任一类型的特殊处理，这些处理可能还会依赖当前方法的局部变量，因此无法统一提到watch中执行。对于这些处理我们怎么能够写得更加优雅呢？
```golang
func func1() (val int, err error) {
    wg := sync.WaitGroup{}
    wg.Add(1)
    defer panics.Always(wg.Done).
        Panic(func(info panics.PanicInfo) {
            err, val = fmt.Errorf("panic with:%v", panicObj), -1
        }).
        Succeed(func() { val = 1 }).
        Recover()
    ...
}
```
可能看起来上面的方法不是很优雅，但是实际大部分不会那么复杂，很多可能只以一行来实现的，比如`defer panics.Always(wg.Done).Recover()`，是不是还比较优雅呢？另外以Always(Ref)/Panic(Ref)/Succeed(Ref)这样明确语义的方法，是不是可读性也更高呢？

## API解读

在panics库中，包括两个实体：
- action：一次recover的执行动作，可以基于action设置异化行为
- settings: 处理recover的配置，基于settings创建出的action会以settings的控制处理为默认

### Action

action的方法：
- `Recover()`: 执行Recover 
- `RecoverWithContext(ctx context.Context)`: 执行Recover，如果发生panic，传入的ctx为随PanicInfo给到Watch方法和Panic处理方法
- `Always(f func()) action`:  传入的方法不管有没有panic都会被执行。如果多次设置该方法，最后一次的值生效
- `AlwaysRef(f *func()) action`: 传入的方法不管有没有panic都会被执行。如果多次设置该方法，最后一次的值生效
- `Succeed(f func()) action`: 传入的方法在**没有**panic时被执行。如果多次设置该方法，最后一次的值生效
- `SucceedRef(f *func()) action`: 传入的方法在**没有**panic时被执行。如果多次设置该方法，最后一次的值生效
- `Panic(f func(PanicInfo)) action`: 传入的方法在**发生**panic时被执行。如果多次设置该方法，最后一次的值生效
- `PanicRef(f *func(PanicInfo)) action`: 传入的方法在**发生**panic时被执行。如果多次设置该方法，最后一次的值生效
- `Alias(alias string) action`: 为当前处理recover的位置设置别名，当这个位置发生panic时，PanicInfo会携带alias给到Watch和Panic处理方法，便于快速发现panic在哪儿被recover
- `Safe(safe bool) action`: 设置通过Always(Ref)/Panic(Ref)/Succeed(Ref)注入的方法的执行方式，如果设置了true。注入方法将以fallbackSettings（不太容易出错）进行Recover。优先级高于settings中safe的优先级
- `WithExtra(any) action`: 当panic时Extra将给到Watch方法和Panic处理方法


action的创建：
- `Use(s *settings) action`: 基于配置创建action
- `ByName(name string) action`: 基于name关联的配置创建action，如果没有发现name关联的配置，使用默认的settings创建action
- 通过`Recover/RecoverWithContext/Always/AlwaysRef/Succeed/SucceedRef/Panic/PanicRef/Alias/Safe/WithExtra`方法，将基于**全局**配置创建出action

### Settings

- `SetWatch(f func(PanicInfo)) *settings`: 设置watch方法，该方法将在发生panic时被调用
- `SetSafe(safe bool) *settings`: 设置通过Always(Ref)/Panic(Ref)/Succeed(Ref)注入的方法的执行方式，如果设置了true。注入方法将以fallbackSettings（不太容易出错）进行Recover
- `SetIgnorePositionChecker(checkers ...ignorePositionChecker) *settings`: 设置堆栈分析时，用于跳过业务不关注的panic位置信息的检测方法。如果checker返回true，表示业务对传入的行信息不关注；
