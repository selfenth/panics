# panics
---

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
    defer panics.AlwaysStatic(wg.Done).
        PanicStatic(func(info panics.PanicInfo) {
            err, val = fmt.Errorf("panic with:%v", panicObj), -1
        }).
        SucceedStatic(func() { val = 1 }).
        Recover()
    ...
}
```

