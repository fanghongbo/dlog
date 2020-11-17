## log for golang
参考 [falcon-log-agent](https://github.com/didi/falcon-log-agent) 项目中的 dlog 代码，按照自己的需求进行了修改
- 可以自定义日志输出目录和文件名称, 且所有日志将写入到一个日志文件中
- 支持同时打印到终端和后端日志文件

#### 常量
- severity
    - DEBUG
    - INFO
    - WARNING
    - ERROR
    - FATAL

#### 格式
2015-06-16 12:00:35 ERROR test.go:12 ...

#### backend
- 实现Log(s Severity, msg []byte) 和 Close()
- 初始时调用`dlog.SetLogging("INFO", backend)`；默认输出到stdout，级别为DEBUG；单独设置日志级别：`dlog.SetSeverity("INFO")`

#### 同时输出到stdout和对应的后端（方便调试用）

    if debug {
        dlog.LogToStdout()
    }

#### log to local file 
    
    b, err := dlog.NewFileBackend("./log", "run.log")  // log文件目录和日志文件名称
    if err != nil {
        panic(err)
    }
    dlog.SetLogging("INFO", b)        // 只输出大于等于INFO的log
    b.Rotate(10, 1024*1024*500)       // 自动切分日志，保留10个文件（INFO.log.000-INFO.log.009，循环覆盖），每个文件大小为500M;
    b.SetFlushDuration(3*time.Second) // 来设置日志延时写入到日志文件
    dlog.Info(1, 2, " test")
    dlog.Close()
   
- log将输出到指定目录下面
- 为了配合op的日志切分工具，有个goroutine定期检查log文件是否消失并且创建新的log文件
- 为了性能使用bufio，bufferSize为256kB。dlog库会自己定期Flush到文件。在主程序退出之前需要调用`dlog.Close()`，否则可能会丢失部分log。

#### logger

    logger := NewLogger("DEBUG", backend)
    logger.Info("asdfasd")
    logger.Close()

#### 输出到多个后端

    b1, err := dlog.NewFileBackend("./log", "run1.log")
    if err != nil {
        return
    }

    b2, err := dlog.NewFileBackend("./log", "run2.log")
    if err != nil {
        return
    }
    
    d , _ := dlog.NewMultiBackend(b1, b2)

    dlog.SetLogging("INFO", d)

    dlog.Info(1, 2, " test") // 日志会同时写入到 ./log/run1.log 和 ./log/run2.log 
    dlog.Close()
    
#### 按小时rotate
* 在配置文件中配置rotateByHour = true
* 如果是使用`b := dlog.NewFileBackend("./log", "run.log")`得到的后端，请调用`b.SetRotateByHour(true)`来开启按小时滚动
* run.log.2016040113, 表示log在2016/04/01, 下午13：00到14：00之间的log, 此log在14：00时生成
* 如果需要定时删除N个小时之前的log，请在配置文件中配置keepHours = N，例如想保留24小时的log，则keepHours = 24
* 如果是使用`b := log.NewFileBackend`得到的后端，请调用`b.SetKeepHours(N)`来指定保留多少小时的log
