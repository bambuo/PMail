package app

// InitChan 控制初始化流程结束
var LifecycleInitChan = make(chan bool)

// RestartChan 控制程序重启
var LifecycleRestartChan = make(chan bool)

// StopChan 控制程序结束
var LifecycleStopChan = make(chan bool)
